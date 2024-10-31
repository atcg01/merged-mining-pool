package rpc

func (r *RPCClient) Check() bool {
	_, err := r.GetBlockTemplate()
	return err == nil
}
