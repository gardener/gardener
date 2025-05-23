// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"strings"

	"k8s.io/utils/clock"

	"github.com/gardener/gardener/pkg/utils"
)

var (
	// GenerateRandomString is an alias for utils.GenerateRandomString. Exposed for testing.
	GenerateRandomString = utils.GenerateRandomString
	// FakeGenerateRandomString is a fake for GenerateRandomString.
	FakeGenerateRandomString = func(n int) (string, error) {
		return strings.Repeat("_", n), nil
	}

	// Read is an alias for crypto/rand.Read. Exposed for testing.
	Read = rand.Read

	// GenerateKey is an alias for rsa.GenerateKey. Exposed for testing.
	GenerateKey = rsa.GenerateKey
	// FakeGenerateKey is a fake for GenerateKey.
	FakeGenerateKey = func(_ io.Reader, _ int) (*rsa.PrivateKey, error) {
		return utils.DecodePrivateKey([]byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAyV6ZuR4gSzCF/zO06xEv6RGmDUnXOHAZVck4pVhY/Id8j2zj
rVlBZp1klARK/Mt1BPOmRKQtg753UCewYjpRdThyzsicKz4Flg4m72p57bWs/wi+
j2N5Rc0eF98Ry//FY6Gbs5VJViz7WSfEoXaSFEYIkv+CKKAQ9J0kkiYztiyz+p/u
SD7sIOAVksj4M5/D+4GVtqJV+4aSdUotoueehJ1fwmc/ZTsczMXAnLcV6BP9N0GX
5bUBW+s/HSMLndEy+GSye1KdgLZilzAodmtetQdLYCOXZsivfdCeF8lsLjLV/ouA
M+FwwM5QbU1i+iYRqVk8Apyzs9WMvuAp8mq5UQIDAQABAoH/O8fZ2xsWezvsi9bN
3vs7PfX/VfKV8itVWiJirrOLt2yBjhLFhLD6uXwAX/DmUiYUl2O9+KLE4FerFCC0
PHUTubkIXFsyAaRoBCQvauQxTmCg+xWdfPQLDK3YQT34CpfkAa/4iVfIbczs0Yr8
1PJea6Ze5UT1Xxol7ni4Yqr0ryAPbJBn+18OifcSxh2H+d7+AEFo/Vg2LVFTiuhW
kpg2xvkmSFjOcIWGUYOlwwnaOjlhiAmCntCAXbz2Ly44rfJlBLzfAAB5CqGzDs2B
Z0YGZoFPQurxkzNGh2d9sV0aHcyf4ZwSbvcsd4gvBhpSp2/Q/mvfdl4av5cKnsli
WJWxAoGBAOqdWcE42I/botGEIfqxssHKyxqQld8RiXjAypPlhx8uRH949sToevZs
BVCgLId8mPJxuTSvbgbdHyZ14dzc+cIcDSNnW8anUTW98lmwTWIJN/awOTSlgpV2
4wBdVCLxlutsE6fEQTIJRkQ+XeVV0n8hOiz4GJQWLV1pp1rzYy73AoGBANu5fKR7
8FXWAfC5zmJAkisK02l7FeRQoHUfgACLE74Vt3BEZhLJHpYTJZrYi9r/buMsi52g
+Rgz4pItgy85ibe21+5G6yQtQP68mjnecMEjSZIa8G6RoY13Ki4+UOysGWul48rR
Lwq75Cv+0AHUS0A9NxYrY+X2Q9cLsg6Mm5/3AoGBAOe38WX9lya+btkv/79ysnLk
sCTUmLFwyK4S/AGGuSX6tHySJGfmlUu89KLlEBXg4c7Ss3FtsuXkj1eVJjbVqXgl
7HQDKYnSx0qlCC+9CTDCmhtzgYyVy5uDiEBb7TV2FvD+FYulMh8ROe09C8/uK7CU
SLkRcHUSUkvohfo2WMeRAoGAa0hK2okFVPPUKLSgV4rNk6SKiyMlEkBnyCgkOJ+v
eQ1jbraG3D9E5uPcZZm716cGfndeiA1z8mRLCTKdre47Fu94yQfpgdVyua5e40h/
512Sa3spz+LdbZQ0jTWyD40MMGpkKcAvZt9MzkpxR6NfRrNc9T8kXMD8aMB2JPJ0
fgsCgYEAzBjM5L4kKcyF5mC1v6NyEaQB8Cve3gfFatLfFrjNwHbvdY5PEa/x0NqS
4qJs/0Ieluo9jRo5pPd0O1u9hDVeSh2sSs9fzOtjHzbnZ7o8pTY3dzMBhO7fxPBU
i/WyG5dokMowEJSvpCBwHbAYMLlNK7oMUpXlqcRoYo24U6Mwj68=
-----END RSA PRIVATE KEY-----`))
	}

	// GenerateVPNKey is an alias for generateVPNKey. Exposed for testing.
	GenerateVPNKey = generateVPNKey
	// FakeGenerateVPNKey is a fake for GenerateVPNKey.
	FakeGenerateVPNKey = func() ([]byte, error) {
		return []byte("key"), nil
	}

	// Clock is an alias for clock.RealClock. Exposed for testing.
	Clock clock.Clock = clock.RealClock{}
)
