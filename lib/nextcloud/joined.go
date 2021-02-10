package nextcloud

import (
	"os"
	"sort"
	"strconv"
)

// ReadDirの結果を、可能ならばauto-split-joinのjoinしたものに変換して返す
// 分割されていないファイルも返り値に含まれる
//
// abc
// abc.0
// abc.1
// のようなjoin後の名前が衝突するような場合、
// "abc": [[abc], [abc.0, abc.1]]
// のようになる。
//
// 返り値: join後のファイル名 -> []元のFileInfoたち
func (n *Nextcloud) ReadJoinedDir(path string) (map[string][][]os.FileInfo, error) {
	fl, err := n.ReadDir(path)
	if err != nil {
		return nil, err
	}

	// flがソート済みかわからないので、ファイル名でソートする
	// ioutil.ReadDirはソートして返して、auto-split-joinはioutil.ReadDirを使っているので。
	sort.Slice(fl, func(i, j int) bool {
		return fl[i].Name() < fl[j].Name()
	})

	result := map[string][][]os.FileInfo{}

	for i := 0; i < len(fl); i++ {
		// 分割ファイルの先頭を探索

		if fl[i].IsDir() {
			result[fl[i].Name()] = append(result[fl[i].Name()], []os.FileInfo{fl[i]})
			continue
		}

		name, ext := splitPathExt(fl[i].Name())

		if !isFilled(ext, '0') {
			// '0' 以外の文字が含まれている
			result[fl[i].Name()] = append(result[fl[i].Name()], []os.FileInfo{fl[i]})
			continue
		}

		fileInfos := []os.FileInfo{fl[i]}

		// 次の分割ファイルを探索
		n := 1
		for ; i+n < len(fl); n++ {
			if fl[i+n].IsDir() {
				// ディレクトリ
				break
			}

			name1, ext1 := splitPathExt(fl[i+n].Name())

			if name1 != name {
				// ファイル名が違う
				break
			}

			if len(ext1) != len(ext) {
				// 桁が違う
				break
			}

			if v, err := strconv.ParseInt(ext1, 10, 64); err != nil || v != int64(n) {
				// 番号が連番じゃない
				break
			}

			// リストに追加
			fileInfos = append(fileInfos, fl[i+n])
		}

		if n < 2 {
			// 分割ファイル風の名前だけど分割ファイルではなかった
			result[fl[i].Name()] = append(result[fl[i].Name()], []os.FileInfo{fl[i]})
			continue
		}

		// 分割ファイルのときだけfl[i].Name()ではなくjoin後の名前(name)がkeyになる
		result[name] = append(result[name], fileInfos)

		i += n - 1
	}

	return result, nil
}

func isFilled(v string, r rune) bool {
	for _, r1 := range v {
		if r1 != r {
			return false
		}
	}
	return true
}

func splitPathExt(path string) (string, string) {
	for i := len(path) - 1; i >= 0 && !os.IsPathSeparator(path[i]); i-- {
		if path[i] == '.' {
			return path[:i], path[i+1:]
		}
	}
	return path, ""
}
