package jwtauth

import (
	"bytes"
	"encoding/json"
)

type MapClaims map[string]interface{}

// UnmarshalJSON 反序列兼容比较长的整数数值
func (m *MapClaims) UnmarshalJSON(data []byte) error {
	type mapClaims MapClaims
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var mc = make(mapClaims, len(*m))
	err := d.Decode(&mc)
	if err != nil {
		return err
	}
	*m = MapClaims(mc)
	return nil
}
