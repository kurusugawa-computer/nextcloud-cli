package webdav

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	_path "path"
	"strconv"
	"strings"
)

type Response struct {
	Href  string
	Props []*Prop
}

type Prop struct {
	Space  string
	Name   string
	Value  string
	Status *Status
}

type Status struct {
	Proto      string
	ProtoMajor int
	ProtoMinor int
	Status     string
	StatusCode int
}

const (
	Depth0        = "0"
	Depth1        = "1"
	DepthInfinity = "infinity"
)

func (n *WebDAV) Propfind(path string, depth string, payload []byte) ([]*Response, error) {
	const MethodPropfind = "PROPFIND"

	path = strings.TrimSuffix(n.URL, "/") + "/" + strings.TrimPrefix(_path.Clean(path), "/")
	req, err := http.NewRequest(MethodPropfind, path, bytes.NewReader(payload))
	if err != nil {
		return nil, &Error{Op: MethodPropfind, Path: path, Type: ErrInvalid, Msg: err.Error()}
	}

	req.Header.Add("Depth", depth)

	req.Header.Add("Content-Type", "text/xml; charset=UTF-8")
	req.Header.Add("Content-Length", strconv.Itoa(len(payload)))

	req.Header.Add("Accept", "application/xml, text/xml")
	req.Header.Add("Accept-Charset", "utf-8")

	if n.AuthFunc != nil {
		n.AuthFunc(req)
	}

	resp, err := n.c.Do(req)
	if err != nil {
		return nil, &Error{Op: MethodPropfind, Path: path, Type: ErrInvalid, Msg: err.Error()}
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusMultiStatus:
		responses := []*Response{}

		d := xml.NewDecoder(resp.Body)
		for {
			token, err := d.Token()
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, &Error{Op: MethodPropfind, Path: path, Type: ErrInvalid, Msg: err.Error()}
			}

			start, ok := token.(xml.StartElement)
			if !ok {
				continue
			}

			if start.Name.Local != "response" {
				continue
			}

			response, err := parseResponse(d, &start)
			if err != nil {
				return nil, &Error{Op: MethodPropfind, Path: path, Type: ErrInvalid, Msg: err.Error()}
			}

			responses = append(responses, response)
		}

		return responses, nil

	case http.StatusForbidden:
		return nil, &Error{Op: MethodPropfind, Path: path, Type: ErrPermission, Msg: resp.Status}

	case http.StatusNotFound:
		return nil, &Error{Op: MethodPropfind, Path: path, Type: ErrNotExist, Msg: resp.Status}

	default:
		return nil, &Error{Op: MethodPropfind, Path: path, Type: ErrInvalid, Msg: resp.Status}
	}
}

type response struct {
	Href     string      `xml:"DAV: href"`
	Propstat []*propstat `xml:"DAV: propstat"`
}

type propstat struct {
	Status string `xml:"DAV: status"`
	Prop   prop   `xml:"DAV: prop"`
}

type prop []propElement

func (prop *prop) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch token := token.(type) {
		case xml.StartElement:
			elem := propElement{}
			if err := d.DecodeElement(&elem, &token); err != nil {
				return err
			}

			*prop = append(*prop, elem)

		case xml.CharData:
			// TODO: error

		case xml.Comment:
		}
	}
}

type propElement struct {
	Space string
	Name  string
	Value string
}

func (p *propElement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.Space = start.Name.Space
	p.Name = start.Name.Local

	token, err := d.Token()
	if err != nil {
		return err
	}

	switch token := token.(type) {
	case xml.CharData:
		p.Value = string(token.Copy())

	case xml.StartElement:
		p.Value = token.Name.Local

	case xml.EndElement:
		p.Value = ""
	}

	for {
		if err := d.Skip(); err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
	}

	return nil
}

func parseResponse(d *xml.Decoder, start *xml.StartElement) (*Response, error) {
	v := response{}
	if err := d.DecodeElement(&v, start); err != nil {
		return nil, err
	}

	response := Response{
		Href: v.Href,
	}

	for _, propstat := range v.Propstat {
		status, err := parseStatus(propstat.Status)
		if err != nil {
			return nil, err
		}

		for _, elem := range propstat.Prop {
			response.Props = append(response.Props, &Prop{
				Space:  elem.Space,
				Name:   elem.Name,
				Value:  elem.Value,
				Status: &status,
			})
		}
	}

	return &response, nil
}

func parseStatus(raw string) (status Status, err error) {
	if i := strings.IndexByte(raw, ' '); i == -1 {
		return status, errors.New("malformed HTTP response: " + raw)

	} else {
		status.Proto = raw[:i]
		status.Status = strings.TrimLeft(raw[i+1:], " ")
	}

	statusCode := status.Status
	if i := strings.IndexByte(status.Status, ' '); i != -1 {
		statusCode = status.Status[:i]
	}
	if len(statusCode) != 3 {
		return status, fmt.Errorf("malformed HTTP status code: %s", statusCode)
	}

	status.StatusCode, err = strconv.Atoi(statusCode)
	if err != nil || status.StatusCode < 0 {
		return status, fmt.Errorf("malformed HTTP status code: %s", statusCode)
	}

	var ok bool
	if status.ProtoMajor, status.ProtoMinor, ok = http.ParseHTTPVersion(status.Proto); !ok {
		return status, fmt.Errorf("malformed HTTP version: %s", status.Proto)
	}

	return status, nil
}
