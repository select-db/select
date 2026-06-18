package utils

import "fmt"

func BytesFromInterface(v interface{}) ([]byte, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case []byte:
		return t, nil
	case string:
		return []byte(t), nil
	default:
		return nil, fmt.Errorf("unsupported type %T", t)
	}
}
