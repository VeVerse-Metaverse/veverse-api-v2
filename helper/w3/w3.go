package w3

import "errors"

func getChainName(chainId string) (err error, chainName string, isMainnet bool) {
	switch chainId {
	case "0x1":
		return nil, "ethereum", true
	case "0x4":
		return nil, "rinkeby", false
	case "0x137":
		return nil, "matic", true
	}

	return errors.New("wrong chain id"), "", false
}

func GetNFTOpenseaLink(chainId string, tokenAddress string, tokenId string) (err error, link string) {
	err, chainName, isMainnet := getChainName(chainId)
	if err != nil {
		return err, link
	}

	if !isMainnet {
		link = "https://testnets.opensea.io/assets/"
	} else {
		link = "https://opensea.io/assets/"
	}

	return nil, link + chainName + "/" + tokenAddress + "/" + tokenId

}
