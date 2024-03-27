nextcloud-cli
====

NextCloud の CLI

WebUI より安心してアップロード/ダウンロードできるかもしれないし、WebUI ではできない再帰的な検索ができるかもしれない。

### インストール

Release ページのアーカイブをダウンロードして任意の場所に展開する。

`.bashrc`等で以下のように`EZA_COLORS`環境変数を設定すると細かく色がつきます

```bash
EZA_COLORS="ur=33;01:uw=31;01:ux=32;01:gr=33:gw=31:gx=32:tr=33:tw=31:tx=32:xx=30;01:sn=32;01:sb=32:uu=33;01:da=34:di=34:lp=36"
```

### 使い方

まずログインする。

```
$ nextcloud-cli login https://kurusugawa.jp/nextcloud/
Enter username: nextcloud-user
Enter password:
```

ファイル一覧を見る。

```
$ nextcloud-cli ls Photos
1403367313553.jpg  Coast.jpg  Hummingbird.jpg  Nut.jpg  user-avatar.png
```

ファイルを検索する。

```
$ nextcloud-cli find Photos -iname *.jpg -or -iname *avatar*
Photos/1403367313553.jpg
Photos/Coast.jpg
Photos/Hummingbird.jpg
Photos/Nut.jpg
Photos/user-avatar.png
```

ファイルやディレクトリをブラウザで開く。

```
$ nextcloud-cli open Photos/user-avatar.png
```

ダウンロードする。

```
$ nextcloud-cli download -o hoge Photos
Photos/1403367313553.jpg 88.14 KiB / 88.14 KiB [=========================] 100.00% 0s
Photos/Coast.jpg 800.55 KiB / 800.55 KiB [===============================] 100.00% 0s
Photos/Hummingbird.jpg 571.50 KiB / 571.50 KiB [=========================] 100.00% 0s
Photos/Nut.jpg 932.64 KiB / 932.64 KiB [=================================] 100.00% 0s
Photos/user-avatar.png 111.13 KiB / 111.13 KiB [=========================] 100.00% 0s
```

単一ファイル（ディレクトリ）をダウンロードする。

```
$ nextcloud-cli download get Photos hoge/fuga.tar
Photos/1403367313553.jpg 88.14 KiB / 88.14 KiB [=========================] 100.00% 0s
Photos/Coast.jpg 800.55 KiB / 800.55 KiB [===============================] 100.00% 0s
Photos/Hummingbird.jpg 571.50 KiB / 571.50 KiB [=========================] 100.00% 0s
Photos/Nut.jpg 932.64 KiB / 932.64 KiB [=================================] 100.00% 0s
Photos/user-avatar.png 111.13 KiB / 111.13 KiB [=========================] 100.00% 0s
```

アップロードする。

```
$ nextcloud-cli upload -o fuga hoge/Photos
hoge/Photos/Hummingbird.jpg 571.50 KiB / 571.50 KiB [====================] 100.00% 0s
hoge/Photos/Nut.jpg 932.64 KiB / 932.64 KiB [============================] 100.00% 0s
hoge/Photos/user-avatar.png 111.13 KiB / 111.13 KiB [====================] 100.00% 0s
hoge/Photos/1403367313553.jpg 88.14 KiB / 88.14 KiB [====================] 100.00% 0s
hoge/Photos/Coast.jpg 800.55 KiB / 800.55 KiB [==========================] 100.00% 0s
```
