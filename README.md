# Fully Go Remote Debugging!

Upload-and-Run at once!

# Usage

```text
Usage of fgor:
  fgor server [...args] : run as server
  fgor exec   [...args] : run program in remote
  
  -type string
        Available:
        go: delve (default)
        java: agent
  -java-agentlib string (default: jdwp=transport=dt_socket,server=y,suspend=n,address=*:5005)
  -connect string
        http address (e.g. 127.0.0.1:2344)
  -continue
        Add --continue flag to remote dlv.
  -delve-listen string
        delve listen address (default "127.0.0.1:2345")
  -listen string
        server listen address (default "127.0.0.1:2344")
  -token string
        authentication token
```

## IDE

- [GoLand](./docs/GoLand.md)

## Server

```
$ fgor server --listen "0.0.0.0:2344" --delve-listen "0.0.0.0:2345" --token "abcd"
```

## Client

```
$ fgor --connect 127.0.0.1:2344 --token "abcd" exec target.exe hello world
```

# Security

TLS-PSK

# License 

See [LICENSE](./LICENSE)

```text
Copyright 2023 JC-Lab (joseph@jc-lab.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```