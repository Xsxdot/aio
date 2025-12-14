package common

import "encoding/json"

type RedisInterface struct {
	V interface{}
}

func (r RedisInterface) MarshalBinary() (data []byte, err error) {
	return json.Marshal(r.V)
}

func (r RedisInterface) UnmarshalBinary(data []byte) (err error) {
	return json.Unmarshal(data, r.V)
}
