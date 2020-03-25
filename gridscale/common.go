package gridscale

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gridscale/gsclient-go/v2"
)

const (
	boolInterfaceType   = "bool"
	intInterfaceType    = "int"
	floatInterfaceType  = "float"
	stringInterfaceType = "string"
)

var supportedPrimTypes = []string{boolInterfaceType, intInterfaceType, floatInterfaceType, stringInterfaceType}

//convSOStrings converts slice of interfaces to slice of strings
func convSOStrings(interfaceList []interface{}) []string {
	labels := make([]string, 0)
	for _, labelInterface := range interfaceList {
		labels = append(labels, labelInterface.(string))
	}
	return labels
}

//convStrToTypeInterface converts a string to a primitive type (in the form of interface{})
func convStrToTypeInterface(interfaceType, str string) (interface{}, error) {
	switch interfaceType {
	case boolInterfaceType:
		return strconv.ParseBool(str)
	case intInterfaceType:
		return strconv.Atoi(str)
	case floatInterfaceType:
		return strconv.ParseFloat(str, 64)
	case stringInterfaceType:
		return str, nil
	default:
		return nil, errors.New("type is invalid")
	}
}

//getInterfaceType gets interface type
func getInterfaceType(value interface{}) (string, error) {
	switch value.(type) {
	case bool:
		return boolInterfaceType, nil
	case int:
		return intInterfaceType, nil
	case float32:
		return floatInterfaceType, nil
	case float64:
		return floatInterfaceType, nil
	case string:
		return stringInterfaceType, nil
	default:
		return "", errors.New("type not found")
	}
}

//convInterfaceToString converts an interface of any primitive types to a  string value
func convInterfaceToString(interfaceType string, val interface{}) (string, error) {
	switch interfaceType {
	case boolInterfaceType:
		v, ok := val.(bool)
		if !ok {
			return "", fmt.Errorf("type assertion error:  value %v is not a bool value", val)
		}
		return strconv.FormatBool(v), nil
	case intInterfaceType:
		v, ok := val.(int)
		if !ok {
			return "", fmt.Errorf("type assertion error:  value %v is not an int value", val)
		}
		return strconv.FormatInt(int64(v), 10), nil
	case floatInterfaceType:
		v, ok := val.(float64)
		if !ok {
			return "", fmt.Errorf("type assertion error:  value %v is not a float64 value", val)
		}
		return strconv.FormatFloat(v, 'f', -1, 32), nil
	case stringInterfaceType:
		v, ok := val.(string)
		if !ok {
			return "", fmt.Errorf("type assertion error:  value %v is not a string value", val)
		}
		return v, nil
	default:
		return "", errors.New("type is invalid")
	}
}

//covStringToMapStringString converts a string to map[string]string
//String format: "key1:val1,key2:val2,key3:val3"
func covStringToMapStringString(str string) (map[string]string, error) {
	formatError := errors.New(`invalid string. valid format: "key1:val1,key2:val2,key3:val3"`)
	result := make(map[string]string)
	//Split string by commas
	commaSplitSlice := strings.Split(str, ",")
	//loop through all "key:value" element in commaSplitSlice
	for _, v := range commaSplitSlice {
		if strings.TrimSpace(v) != "" {
			//Split the element by a colon
			colonSplitSlice := strings.Split(v, ":")
			//the length of colonSplitSlice has to be 2 as
			//there are 2 elements: key and value
			if len(colonSplitSlice) == 2 {
				result[colonSplitSlice[0]] = colonSplitSlice[1]
			} else {
				return nil, formatError
			}
		} else {
			//if there is a empty element, return error
			return nil, formatError
		}
	}
	return result, nil
}

//getProjectClientFromMeta get gs client from meta by project's name
func getProjectClientFromMeta(projectName string, meta interface{}) (*gsclient.Client, error) {
	projectsClients, ok := meta.(map[string]*gsclient.Client)
	if !ok {
		return nil, fmt.Errorf("project %s: cannot convert meta to map[string]*gsclient.Client", projectName)
	}
	client, ok := projectsClients[projectName]
	if !ok {
		return nil, fmt.Errorf("project %s's client has not been configured", projectName)
	}
	return client, nil
}
