package monitor

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

//Check JSON request body for the required fields and type-checks the values
func validArgs(args map[string]interface{}, required []string) (map[string]float64, map[string]string, error) {

	numArgs := make(map[string]float64)
	strArgs := make(map[string]string)

	for _, k := range required {
		if _, ok := args[k]; ok {

			switch k {

			case "id":
				if id, ok := args[k].(float64); ok {
					if id < 0 {
						return nil, nil, fmt.Errorf("Parameter 'id' negative value: %d", int(id))
					}
					numArgs[k] = id
				} else {
					return nil, nil, fmt.Errorf("Parameter 'id' not numerical")
				}

			case "frequency":
				if frequency, ok := args["frequency"].(float64); ok {
					if frequency < 60 {
						return nil, nil, fmt.Errorf("Parameter 'frequency' minimum 60, has %d", int(frequency))
					}
					numArgs["frequency"] = frequency
				} else {
					return nil, nil, fmt.Errorf("Parameter 'frequency' not numerical")
				}

			case "timeout":
				if timeout, ok := args["timeout"].(float64); ok {
					if timeout < 1 || timeout > 60 {
						return nil, nil, fmt.Errorf("Parameter 'timeout' allowed range(1, 60), has %d", int(timeout))
					}
					numArgs["timeout"] = timeout
				} else {
					return nil, nil, fmt.Errorf("Parameter 'timeout' not numerical")
				}

			case "url":
				if url, ok := args["url"].(string); ok {
					strArgs["url"] = url

				} else {
					return nil, nil, fmt.Errorf("Parameter 'url' not a string")
				}

			default:
				continue

			}

		} else {
			return nil, nil, fmt.Errorf("Missing parameter '%s'", k)
		}
	}

	return numArgs, strArgs, nil
}

func unmarshalRequestBody(body io.ReadCloser) (map[string]interface{}, error) {

	f := make(map[string]interface{})
	b, err := ioutil.ReadAll(body)

	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}

	return f, nil
}

func marshalResponseBody(f interface{}) ([]byte, error) {
	return json.Marshal(f)
}
