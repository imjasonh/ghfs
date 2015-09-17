# GHFS - GitHub repos in your filesystem!

GHFS mounts to a directory using FUSE and serves contents of GitHub repos
on-demand.

Let's try an example. First, generate a GitHub personal access token
[here](https://github.com/settings/tokens) -- it only needs the
`public_repo` scope.

```
$ mkdir /github
$ go run main.go -token=$GITHUB_TOKEN -mountpoint=/github
2015/09/17 04:27:35 serving...
```

While that runs, in another terminal, list repos owned by a user or organization:

```
$ ls /github/golang/
appengine
arch
benchmarks
blog
...
```

You can inspect a repo's branches and tags as subdirectories of the repo:

```
$ ls /github/golang/go
...
master
...
```

And you can inspect the directories and files in the repo at those branches or
tags:

```
$ ls /github/golang/go/master
AUTHORS
CONTRIBUTING.md
CONTRIBUTORS
LICENSE
...
```

If you know the specific revision you want to explore, you can use that instead of a branch or tag name:

```
$ ls /github/golang/go/3d1f8c237956ca657b9517040a7431e87f9d8a18
AUTHORS
CONTRIBUTING.md
CONTRIBUTORS
LICENSE
...
```

Or use any unique prefix of the revision SHA:

```
$ ls /github/golang/go/3d1f8c23
```

At a revision/branch/tag, you can explore the repo and read files:

```
$ cat /github/golang/go/master/src/bytes/bytes.go
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bytes implements functions for the manipulation of byte slices.
// It is analogous to the facilities of the strings package.
package bytes

import (
	"unicode"
	"unicode/utf8"
)
...
```

```
$ wc -l /github/golang/go/master/src/bytes/bytes.go
     714 /github/golang/go/master/src/bytes/bytes.go
```

```
$ grep -n TODO /github/golang/go/master/src/bytes/bytes.go
429:// TODO: update when package unicode captures more of the properties.
```

```
$ diff \
  /github/golang/go/89454b1c/src/bytes/bytes.go \
  /github/golang/go/3d1f8c23/src/bytes/bytes.go
140a141,150
> // LastIndexByte returns the index of the last instance of c in s, or -1 if c is not present in s.
> func LastIndexByte(s []byte, c byte) int {
> 	for i := len(s) - 1; i >= 0; i-- {
> 		if s[i] == c {
> 			return i
> 		}
> 	}
> 	return -1
> }
> 
```

----------

License
-----

    Copyright 2015 Jason Hall

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.

