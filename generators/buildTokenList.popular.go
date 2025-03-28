package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/migratooor/tokenLists/generators/common/chains"
	"github.com/migratooor/tokenLists/generators/common/helpers"
	"github.com/migratooor/tokenLists/generators/common/logs"
	"github.com/migratooor/tokenLists/generators/common/models"
	"github.com/migratooor/tokenLists/generators/common/utils"
)

func buildPopularList() {
	tokenList := helpers.LoadTokenListFromJsonFile(`popular.json`)
	tokenList.Name = `Popular tokens`
	tokenList.LogoURI = `https://raw.githubusercontent.com/smoldapp/tokenLists/main/.github/tokenlistooor.svg`
	tokenList.Description = `A curated list of popular tokens from all the token lists on tokenlistooor.`

	/**************************************************************************
	** Create a map of all tokens from all lists and only add the missing ones
	** in it. Map are WAY faster than arrays fir our use case
	**************************************************************************/
	allTokens := make(map[uint64]map[string]models.TokenListToken)
	allTokensPlain := []models.TokenListToken{}
	listsPerChain := make(map[uint64][]string)

	for _, chainID := range chains.SUPPORTED_CHAIN_IDS {
		chain := chains.CHAINS[chainID]
		allTokensPlain = append(allTokensPlain, chain.Coin)
	}

	/**********************************************************************************************
	** First, we want to calculate each list weight. Absolute weight cannot work because some lists
	** are very smol and thus break the 50% rule.
	** To do this, we need to know the total number of tokens in all lists.
	**********************************************************************************************/
	totalNumberOfTokens := 0
	for name, generatorData := range GENERATORS {
		if generatorData.Exclude {
			continue
		}
		if name == `tokenlistooor` {
			continue
		}
		if generatorData.GeneratorType == GeneratorPool {
			continue
		}
		tokenList := helpers.LoadTokenListFromJsonFile(name + `.json`)
		totalNumberOfTokens += len(tokenList.Tokens)
	}

	/**********************************************************************************************
	** Now we can calculate the weight of each list. We will use the number of tokens in the list
	** divided by the total number of tokens in all lists.
	**********************************************************************************************/
	sumWeights := 0.0
	listWeights := make(map[string]float64)
	for name, generatorData := range GENERATORS {
		if generatorData.Exclude {
			continue
		}
		if name == `tokenlistooor` {
			continue
		}
		if generatorData.GeneratorType == GeneratorPool {
			continue
		}
		tokenList := helpers.LoadTokenListFromJsonFile(name + `.json`)
		listWeights[name] = float64(len(tokenList.Tokens)) / float64(totalNumberOfTokens)
		sumWeights += listWeights[name]
		logs.Info(`List weight for ` + name + ` is ` + strconv.FormatFloat(listWeights[name], 'f', -1, 64))
	}

	/**********************************************************************************************
	** Once we have this, we can calculate the weighted frequency: for each token, we calculate its
	** weighted frequency across all lists.
	** So, everytime a token is found in a list, we increment its frequency by the weight of the
	** list.
	**********************************************************************************************/
	for name, generatorData := range GENERATORS {
		if generatorData.Exclude {
			continue
		}
		if name == `tokenlistooor` {
			continue
		}
		if generatorData.GeneratorType == GeneratorPool {
			continue
		}

		tokenList := helpers.LoadTokenListFromJsonFile(name + `.json`)
		for _, token := range tokenList.Tokens {
			if !chains.IsChainIDSupported(token.ChainID) {
				continue
			}
			if _, ok := listsPerChain[token.ChainID]; !ok {
				listsPerChain[token.ChainID] = []string{}
			}

			if !helpers.Includes(listsPerChain[token.ChainID], name) {
				listsPerChain[token.ChainID] = append(listsPerChain[token.ChainID], name)
			}

			if _, ok := allTokens[token.ChainID]; !ok {
				allTokens[token.ChainID] = make(map[string]models.TokenListToken)
			}

			if existingToken, ok := allTokens[token.ChainID][utils.ToAddress(token.Address)]; ok {
				newOccurence := existingToken.OccurrenceFloat
				if newOccurence != math.MaxInt32 {
					foundInExtraTokens := false
					for _, extraToken := range chains.CHAINS[token.ChainID].ExtraTokens {
						if strings.EqualFold(token.Address, extraToken) {
							foundInExtraTokens = true
							break
						}
					}
					if foundInExtraTokens {
						newOccurence = math.MaxInt32
					} else if strings.HasSuffix(name, `-static`) {
						newOccurence = math.MaxInt32
					} else {
						newOccurence += listWeights[name]
					}
				} else {
					newOccurence += listWeights[name]
				}
				allTokens[token.ChainID][utils.ToAddress(token.Address)] = models.TokenListToken{
					Address:         existingToken.Address,
					Name:            helpers.SafeString(existingToken.Name, token.Name),
					Symbol:          helpers.SafeString(existingToken.Symbol, token.Symbol),
					LogoURI:         helpers.SafeString(existingToken.LogoURI, token.LogoURI),
					Decimals:        helpers.SafeInt(existingToken.Decimals, token.Decimals),
					ChainID:         token.ChainID,
					OccurrenceFloat: newOccurence,
				}
			} else {
				tokenInitialOccurence := listWeights[name]
				for _, extraToken := range chains.CHAINS[token.ChainID].ExtraTokens {
					if strings.EqualFold(token.Address, extraToken) {
						tokenInitialOccurence = math.MaxInt32
					}
				}
				if strings.HasSuffix(name, `-static`) {
					tokenInitialOccurence = math.MaxInt32
				}

				allTokens[token.ChainID][utils.ToAddress(token.Address)] = models.TokenListToken{
					Address:         utils.ToAddress(token.Address),
					Name:            helpers.SafeString(token.Name, ``),
					Symbol:          helpers.SafeString(token.Symbol, ``),
					LogoURI:         helpers.SafeString(token.LogoURI, ``),
					Decimals:        helpers.SafeInt(token.Decimals, 18),
					ChainID:         token.ChainID,
					OccurrenceFloat: tokenInitialOccurence,
				}
			}
		}
	}

	/**********************************************************************************************
	** Now that we have the weighted frequency of each token, we can filter out the tokens that
	** have a frequency below 50% to only keep the popular ones.
	**********************************************************************************************/
	for chainID, tokens := range allTokens {
		for _, token := range tokens {
			if _, ok := listsPerChain[chainID]; !ok {
				continue
			}
			weightedThreshold := chains.CHAINS[chainID].WeightRatio * float64(sumWeights)
			if token.OccurrenceFloat >= weightedThreshold {
				allTokensPlain = append(allTokensPlain, token)
			}
		}
	}

	tokens := helpers.GetTokensFromList(allTokensPlain)
	for _, token := range allTokensPlain {
		for i, t := range tokens {
			if common.HexToAddress(token.Address).Hex() == common.HexToAddress(t.Address).Hex() {
				tokens[i].OccurrenceFloat = token.OccurrenceFloat
			}
		}
	}

	helpers.SaveTokenListInJsonFile(tokenList, tokens, `popular.json`, helpers.SavingMethodStandard)

	// Update the chainlist with the new tokens
	tokenList = helpers.LoadTokenListFromJsonFile(`popular.json`)

	// Create the chainList
	helpers.SaveChainListInJsonFile(tokenList)
}
