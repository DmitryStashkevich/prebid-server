package openrtb2

import (
	"encoding/json"
	"github.com/mxmCherry/openrtb"
)

func bidRequestToString(request *openrtb.BidRequest) string {
	if b, err := json.Marshal(request); err == nil {
		return string(b)
	}
	return "Json parse error for BidRequest "
}

func bidResponseToString(response *openrtb.BidResponse) string {
	if b, err := json.Marshal(response); err == nil {
		return string(b)
	}
	return "Json parse error for BidResponse "
}
