package taskengine

import encodingjson "encoding/json"

var json = struct {
	Unmarshal func([]byte, any) error
}{
	Unmarshal: encodingjson.Unmarshal,
}
