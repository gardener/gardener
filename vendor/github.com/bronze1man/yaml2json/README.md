Description
===================
Transform yaml string to json string without the type infomation.

Features
====================
* zero config.
* supports Windows, Linux, macOS, FreeBSD, NetBSD, OpenBSD, Plan 9 etc..

Binary installation and usage
====================
* Download a binary that match your operation system and platform.
* https://github.com/bronze1man/yaml2json/releases
* copy it to `/usr/local/bin` like (`cp ~/Downloads/yaml2json_darwin_amd64 /usr/local/bin/yaml2json` )
* Use `chmod +x /usr/local/bin/yaml2json` give running permission to it.

### mac/linux usage
* `echo "a: 1" | yaml2json`
* `yaml2json < 1.yml > 2.json`

### window usage
* windows 7 cmd.exe:
```
C:\tmp>more .\1.yaml
a: 1

C:\tmp>.\yaml2json_windows_amd64.exe < ./1.yaml > 2.json

C:\tmp>more .\2.json
{"a":1}
```

* windows 7 powerShell 6.1.7600 file example:
```
PS C:\tmp> more .\1.yaml
a: 1

PS C:\tmp> Get-Content .\1.yaml | .\yaml2json_windows_amd64.exe > .\2.json
PS C:\tmp> more .\2.json
{"a":1}
```

Library installation
====================
* `go get -v github.com/bronze1man/yaml2json/y2jLib`


Development
==================
* Follow example is for develop on mac and linux. It should work to windows too. (need change some command line and path to work on windows).
* You need a golang on your computer. https://golang.org
* Create a new directory as your workspace, like `~/work/yaml2json`
* Change your current work directory to that directory.

```
cd ~/work/yaml2json
GOPATH=`pwd` go get -v github.com/bronze1man/yaml2json
GOPATH=`pwd` go run github.com/bronze1man/yaml2json/y2jBuilder
```
* use the file at $GOPATH/tmp/file to distribute your binary to others.



Notice
=====================
* if you don't know whether your platform is 386 or amd64, use the 386 build...
* Library api is not easy to use, it will better in next version.

Reference
====================
* https://github.com/peter-edge/go-yaml2json
* https://github.com/go-yaml/yaml
* https://github.com/bronze1man/json2yaml
