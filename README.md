# Fully Go Remote Debugging!

Upload-and-Run at once!

# Usage

## Server

```
$ fgor server -listen "0.0.0.0:2344" -delve-listen "0.0.0.0:2345" -token "abcd"
```

## Client

```
$ fgor --connect 127.0.0.1:2344 exec target.exe hello world
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