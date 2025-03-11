package internal

//type MemberAddedPayload struct {
//	Channel    constants.ChannelName   `json:"channel"`
//	MemberData pusherClient.MemberData `json:"memberData"`
//}

//// publishPresenceMemberAdded sends a message to pubsub to announce that a member has been added to a presence channel
//func publishPresenceMemberAdded(_payload MemberAddedPayload) error {
//	payload, err := json.Marshal(_payload)
//	if err != nil {
//		return errors.New("error marshalling member added payload")
//	}
//
//	m := pubsub.ServerMessage{
//		NodeID:  GlobalHub.nodeID,
//		Event:   constants.SocketRushEventPresenceMemberAdded,
//		Payload: payload,
//	}
//	GlobalHub.sendToOtherServers(m)
//	return nil
//}
