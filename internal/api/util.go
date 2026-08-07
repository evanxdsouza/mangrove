package api

import "strconv"

func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
