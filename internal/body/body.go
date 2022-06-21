package body

import (
	"io"
	"io/ioutil"

	"github.com/rs/zerolog/log"
)

func GetBody(readerBody io.ReadCloser) *[]byte {
	defer readerBody.Close()

	respBody, err := ioutil.ReadAll(readerBody)
	if err != nil {
		log.Error().Msgf("Couldn't read body %v\n", err)
		return nil
	}
	return &respBody
}
