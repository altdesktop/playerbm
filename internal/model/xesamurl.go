package model

import (
	"github.com/kballard/go-shellquote"
	urllib "net/url"
)

type XesamUrl struct {
	base *urllib.URL
}

func ParseXesamUrl(xesamUrl string) (*XesamUrl, error) {
	url, err := urllib.Parse(xesamUrl)
	if err != nil {
		return nil, err
	}

	if url.Scheme == "" {
		url.Scheme = "file"
	}

	return &XesamUrl{base: url}, nil
}

func (xesamUrl *XesamUrl) Path() string {
	return xesamUrl.base.Path
}

func (xesamUrl *XesamUrl) String() string {
	return xesamUrl.base.String()
}

func (xesamUrl *XesamUrl) Scheme() string {
	return xesamUrl.base.Scheme
}

func (xesamUrl *XesamUrl) ShellQuoted() (string, error) {
	var urlString string
	if xesamUrl.base.Scheme == "file" {
		urlString = xesamUrl.base.Path
	} else {
		urlString = xesamUrl.base.String()
	}
	unescaped, err := urllib.PathUnescape(urlString)
	if err != nil {
		return "", err
	}
	return shellquote.Join(unescaped), nil
}
