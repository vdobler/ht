// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/base64"
	"testing"
)

var imgr = Response{BodyStr: "\x89\x50\x4e\x47\x0d\x0a\x1a\x0a\x00\x00\x00\x0d\x49\x48\x44\x52" +
	"\x00\x00\x00\x08\x00\x00\x00\x06\x08\x06\x00\x00\x00\xfe\x05\xdf" +
	"\xfb\x00\x00\x00\x01\x73\x52\x47\x42\x00\xae\xce\x1c\xe9\x00\x00" +
	"\x00\x06\x62\x4b\x47\x44\x00\x00\x00\x00\x00\x00\xf9\x43\xbb\x7f" +
	"\x00\x00\x00\x34\x49\x44\x41\x54\x08\xd7\x85\x8e\x41\x0e\x00\x20" +
	"\x0c\xc2\x28\xff\xff\x33\x9e\x30\x6a\xa2\x72\x21\xa3\x5b\x06\x49" +
	"\xa2\x87\x2c\x49\xc0\x16\xae\xb3\xcf\x8b\xc2\xba\x57\x00\xa8\x1f" +
	"\xeb\x73\xe1\x56\xc5\xfa\x68\x00\x8c\x59\x0d\x11\x87\x39\xe4\xc3" +
	"\x00\x00\x00\x00\x49\x45\x4e\x44\xae\x42\x60\x82"}

var imgl = Response{BodyStr: string(mustDecodeBase64(imglBody64))}
var imglBody64 = `/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAA0JCgsKCA0LCgsODg0PEyAVExISEyccHhcgLikxMC4p
LSwzOko+MzZGNywtQFdBRkxOUlNSMj5aYVpQYEpRUk//2wBDAQ4ODhMREyYVFSZPNS01T09PT09P
T09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT0//wAARCABAAEADASIA
AhEBAxEB/8QAGAABAQEBAQAAAAAAAAAAAAAABQQGAwL/xAAwEAACAQMCBAMIAgMBAAAAAAABAgMA
BBEFIRIxQWEUIlEGEzJCcYGRobHRJGLB4f/EABkBAQEBAQEBAAAAAAAAAAAAAAIDAQQABf/EACAR
AAIDAAEEAwAAAAAAAAAAAAABAgMRIRIiMXEyQlH/2gAMAwEAAhEDEQA/AEZCBHIAfMF37bUDbHJX
6Vdp8rTQ3kjnzNufxR1pvwAddq5IrNR95CINdUt55N0QEfUZqVctIRnAU4pnTFbduSHkT1+lJIFk
3FagmUFeJXBUjmDzFV2jg2ycW5A50xfafFf25QYSYfBJ6dj2rNXTzWWlhuAiaKQKyc8niwRXnHeA
wtU16F41W8heyJ8x88JPR/T71k9T1Foy1vBlXU4duoPoP7pbW57jTrGJxG8Mlx8DHYptk/Q70Nom
meNlMswPuVPX5jTqhi6pBlJb2sd0kf41zn0/5UOmb3dqp5GRR+6v0g5iuh2/5RdqpPBwk8XTHr0q
a8suI+6EVzIp3AblVi6jFBhJHwTROrakPeNJEAWk5f69Mn75o/TLSbVLoYZhg5kfngVaFfbsiFkk
+DY2OpJeOyWzFynxNggD/wBqyRreDFxdMgbozDcnsPWotO8PGFtYGQLHzA5k+tEe0NxJa6nP4Sbz
Pb8QYHJRgQCO21HE3iIdDb/CX2nvW1W+tLGIYKZLDqCx5HuAP3TFtAlrbJDGMKorP+zdtx3Uly24
j2BPU9a0juOox3rbH9UOKSDPZ+QSLej0A/g1AkjQRxmHAlccOTuFB649at9lzx2ty55t/VEswe4i
RXwAucn1G9bVFOxplbJZHgmcmaGfhDcSvkFjnPTHcnNPacsUMDaYDkkZuXB3Zj8ue22ahnaGyg8Q
APGO/k2+DG/EO+/P7Vx0a5SKQq7buc5J5mqW60wVJNrTTWltb2l7bRwALnykqMZznBPfes7fQRw6
o0cfWBxjvg/vanhKY1a4lSMpGCQxHL91mYJXu9TE7DHHMB9Adsfg1KveWUmkuB3RYfD6XFkeZxxn
70vZ2fiV95Jnh+VeWe9RuBHEANgq4FaG2RYrSLOAAgJ/FRnJ+SEniMpoUcSPKYAVimgSUKemc5H6
rNNORczSj5XOMnpnGPxWp0VFSzgkQkobUqGPXhY/3WRCmWVyuwZjjtvXVV85GyexQlLi500OcFlz
j8ch/FT6ZAWnVyNg/Dv9DXOUv4NYkB4y3CQP0cdDTGmWzRInGMBRnGckseZNK+eIVENYzZcGeB1V
lIwVYZH4qfVdFhtLi0vLFOCKSYLLGOSnGQR6DblXSPKsGFKyA3WkTxKSH4CVI5gjcVyQk9FcsakD
x2jXFwsELspbOd9sV49prprm6NoJD4eHAKj5279hXDT76W2aS5mzMFUGEAYOe59KKmmkaOSaVsu5
xn1Y+lNRbl6MSzln/9k=`

func mustDecodeBase64(s string) []byte {
	t, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return t
}

var imageTests = []TC{
	{imgr, Image{Format: "png"}, nil},
	{imgr, Image{Format: "png", Width: 8}, nil},
	{imgr, Image{Format: "png", Height: 6}, nil},
	{imgr, Image{Format: "jpg"}, errCheck},
	{imgr, Image{Format: "png", Width: 12}, errCheck},
	{imgr, Image{Format: "png", Height: 8}, errCheck},
	{imgr, Image{Format: "png",
		Fingerprint: "000000000000000000f00007", Threshold: 0.01}, nil},
	{imgl, Image{Format: "jpeg", Fingerprint: "4f000000f400006010040004", Threshold: 0.01}, nil},
	{imgl, Image{Format: "jpeg", Fingerprint: "b698bd890b0b8f8c", Threshold: 0.01,
		Width: 64, Height: 64}, nil},
	{imgl, Image{Fingerprint: "bababuba"}, errDuringPrepare},
	{imgl, Image{Fingerprint: "4f000000f40000601004000"}, errDuringPrepare},
	{imgl, Image{Fingerprint: "4f000000f40000601004000ff"}, errDuringPrepare},
	{imgl, Image{Fingerprint: "strange"}, errDuringPrepare},
}

func TestImage(t *testing.T) {
	for i, tc := range imageTests {
		runTest(t, i, tc)
	}
}
