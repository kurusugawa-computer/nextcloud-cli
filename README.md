nextcloud-cli
====

NextCloud の CLI

WebUI より安心してアップロード/ダウンロードできるかもしれない。

### インストール

Release ページのアーカイブをダウンロードして任意の場所に展開する。

### 使い方

まずログインする。

```
$ nextcloud-cli login https://kurusugawa.jp/nextcloud/
```

ファイル一覧を見る。

```
$ nextcloud-cli ls Photos
-rw-rw-r--	  90 kB	2017-09-15 05:38	1403367313553.jpg
-rw-rw-r--	 820 kB	2017-09-15 03:10	Coast.jpg
-rw-rw-r--	 585 kB	2017-09-15 03:10	Hummingbird.jpg
-rw-rw-r--	 955 kB	2017-09-15 03:10	Nut.jpg
-rw-rw-r--	 114 kB	2018-05-25 07:11	user-avatar.png
```

ファイルを検索する。

```
$ nextcloud-cli find --ls Photos -iname jpg -or -iname avatar
-rw-rw-r--	  90 kB	2017-09-15 05:38	Photos/1403367313553.jpg
-rw-rw-r--	 820 kB	2017-09-15 03:10	Photos/Coast.jpg
-rw-rw-r--	 585 kB	2017-09-15 03:10	Photos/Hummingbird.jpg
-rw-rw-r--	 955 kB	2017-09-15 03:10	Photos/Nut.jpg
-rw-rw-r--	 114 kB	2018-05-25 07:11	Photos/user-avatar.png
```

ディレクトリをブラウザで開く。

```
$ nextcloud-cli open Photos
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

アップロードする。

```
$ nextcloud-cli upload -o fuga hoge/Photos
hoge/Photos/Hummingbird.jpg 571.50 KiB / 571.50 KiB [====================] 100.00% 0s
hoge/Photos/Nut.jpg 932.64 KiB / 932.64 KiB [============================] 100.00% 0s
hoge/Photos/user-avatar.png 111.13 KiB / 111.13 KiB [====================] 100.00% 0s
hoge/Photos/1403367313553.jpg 88.14 KiB / 88.14 KiB [====================] 100.00% 0s
hoge/Photos/Coast.jpg 800.55 KiB / 800.55 KiB [==========================] 100.00% 0s
```
