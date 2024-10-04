package main

import "fmt"

func encodeBencode(obj any) ([]byte, error) {
	switch obj.(type) {
	case string:
		str := obj.(string)
		return []byte(fmt.Sprintf("%d:%s", len(str), str)), nil
	case int:
		i := obj.(int)
		return []byte(fmt.Sprintf("i%de", i)), nil
	case []any:
		ls := obj.([]any)
		v := "l"
		for _, o := range ls {
			iv, err := encodeBencode(o)
			if err != nil {
				return nil, err
			}
			v += string(iv)
		}
		v += "e"
		return []byte(v), nil
	case map[string]any:
		m := obj.(map[string]any)
		v := "d"
		for key, value := range m {
			kv, err := encodeBencode(key)
			if err != nil {
				return nil, err
			}
			v += string(kv)
			vv, err := encodeBencode(value)
			if err != nil {
				return nil, err
			}
			v += string(vv)
		}
		v += "e"
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unhandled type: %T", obj)
	}
}
