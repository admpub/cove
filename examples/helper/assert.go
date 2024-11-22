package helper

import (
	_ "github.com/admpub/cove/driver"
)

func AssertNoErr(err error) {
	if err != nil {
		panic(err)
	}
}
