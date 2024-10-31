package pool

func isBanned(ip string) bool {
	if len(ip) > 0 {
		return false // TODO
	}
	return false
}

// func surpassedLimitPolicy(ip string) bool {
// 	if len(ip) > 0 {
// 		return false // TODO
// 	}
// 	return false
// }

func banClient(client *stratumClient) {
	removeSession(client.sessionID)

	// BAN IP?  BAN Miner address?
}

func markMalformedRequest(client *stratumClient, jsonPayload []byte) {

}
