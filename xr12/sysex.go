package xr12

import "fmt"

var SysexPrefix = []byte{0xF0, 0x00, 0x20, 0x32, 0x32}
var SysexSufix =  []byte{0xF7}


func appendOscPrefixAndSuffic(cmd []byte) []byte {
	cmd = append(SysexPrefix, cmd...)
	cmd = append(cmd, SysexSufix...)

	return cmd
}


func concatAppend(slices [][]byte) []byte {
	var tmp []byte
	for _, s := range slices {
		tmp = append(tmp, s...)
	}
	return tmp
}

func printHex(x []byte) {
	fmt.Printf("%s\n", fmt.Sprintf("% X", x))
}