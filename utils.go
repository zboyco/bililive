package bililive

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
)

func httpSend(url string) ([]byte, error) {
	tr := &http.Transport{ //解决x509: certificate signed by unknown authority
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
