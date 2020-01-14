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

func (xesamUrl *XesamUrl) UnescapedPath() string {
	var path string
	if xesamUrl.base.Scheme == "file" {
		path = xesamUrl.String()[len("file://"):]
	} else {
		path = xesamUrl.base.Path
	}
	unescaped, err := urllib.PathUnescape(path)

	if err != nil {
		// xesamUrl type is valid by construction
		panic(err)
	}

	return unescaped
}

func (xesamUrl *XesamUrl) String() string {
	return xesamUrl.base.String()
}

func (xesamUrl *XesamUrl) Scheme() string {
	return xesamUrl.base.Scheme
}

func (xesamUrl *XesamUrl) ShellQuoted() string {
	urlString := xesamUrl.base.String()
	if xesamUrl.base.Scheme == "file" {
		urlString = urlString[len("file://"):]
	}

	unescaped, err := urllib.PathUnescape(urlString)
	if err != nil {
		// xesamUrl type is valid by construction
		panic(err)
	}
	return shellquote.Join(unescaped)
}
