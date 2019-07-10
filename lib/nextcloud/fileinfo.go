package nextcloud

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	_path "path"
	"strconv"
	"time"

	"github.com/kurusugawa-computer/nextcloud-cli/lib/webdav"
)

var propfind = []byte(`<d:propfind xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns">
<d:prop>
	<d:displayname/>
	<d:getcontentlength/>
	<d:getlastmodified/>
	<d:resourcetype/>
	<oc:permissions/>
	<oc:id/>
	<oc:owner-id/>
	<oc:owner-display-name/>
</d:prop>
</d:propfind>`)

func fileInfo(response *webdav.Response) (os.FileInfo, error) {
	fi := FileInfo{
		name:    "",
		size:    0,
		mode:    0664,
		modTime: time.Unix(0, 0),
		isDir:   false,
	}

	href, err := url.QueryUnescape(response.Href)
	if err != nil {
		return nil, err
	}

	fi.name = _path.Base(href)

	for _, prop := range response.Props {
		if prop.Status.StatusCode != http.StatusOK {
			continue
		}

		switch prop.Space {
		case "DAV:":
			switch prop.Name {
			case "displayname":
				fi.name = prop.Value

			case "getcontentlength":
				v, err := strconv.ParseInt(prop.Value, 10, 64)
				if err != nil {
					return nil, err
				}
				if v < 0 {
					return nil, errors.New("getcontentlength must >= 0")
				}

				fi.size = v

			case "getlastmodified":
				v, err := time.Parse(time.RFC1123, prop.Value)
				if err != nil {
					return nil, err
				}

				fi.modTime = v

			case "resourcetype":
				if prop.Value == "collection" {
					fi.isDir = true
					fi.mode = 0775 | os.ModeDir
				} else {
					fi.isDir = false
					fi.mode = 0664
				}
			}

		case "http://owncloud.org/ns":
			switch prop.Name {
			case "permissions":
				fi.permissions = prop.Value

			case "id":
				fi.id = prop.Value

			case "owner-id":
				fi.ownerID = prop.Value

			case "owner-display-name":
				fi.ownerDisplayName = prop.Value
			}
		}
	}

	return &fi, nil
}

type FileInfo struct {
	// DAV:
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool

	// http://owncloud.org/ns
	permissions      string
	id               string
	ownerID          string
	ownerDisplayName string
}

func (f *FileInfo) Name() string {
	return f.name
}

func (f *FileInfo) Size() int64 {
	return f.size
}

func (f *FileInfo) Mode() os.FileMode {
	return f.mode
}

func (f *FileInfo) ModTime() time.Time {
	return f.modTime
}

func (f *FileInfo) IsDir() bool {
	return f.isDir
}

func (f *FileInfo) Sys() interface{} {
	return nil
}

func (f *FileInfo) Permissions() string {
	return f.permissions
}

func (f *FileInfo) ID() string {
	return f.id
}

func (f *FileInfo) OwnerID() string {
	return f.ownerID
}

func (f *FileInfo) OwnerDisplayName() string {
	return f.ownerDisplayName
}
