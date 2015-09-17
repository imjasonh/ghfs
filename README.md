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
$ cd /github
$ ls golang/
appengine
arch
benchmarks
blog
...
```

You can inspect a repo's branches and tags as subdirectories of the repo:

```
$ ls golang/go
master
```

And you can inspect the directories and files in the repo at those branches or
tags:

```
$ ls golang/go/master
AUTHORS
CONTRIBUTING.md
CONTRIBUTORS
LICENSE
...
```

If you know the specific revision you want to explore, you can use that instead of a branch or tag name:

```
$ ls golang/go/3d1f8c237956ca657b9517040a7431e87f9d8a18
AUTHORS
CONTRIBUTING.md
CONTRIBUTORS
LICENSE
...
```

At a revision/branch/tag, you can explore the repo and read files:

```
$ cat golang/go/master/src/bytes/bytes.go
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
$ wc -l golang/go/master/src/bytes/bytes.go
     714 golang/go/master/src/bytes/bytes.go
```

```
$ grep -n TODO golang/go/master/src/bytes/bytes.go
429:// TODO: update when package unicode captures more of the properties.
```
