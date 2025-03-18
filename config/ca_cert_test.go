package config

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestGetCACertTLSCfg_CustomCA(t *testing.T) {
	caCert := `-----BEGIN CERTIFICATE-----
MIIFtzCCA5+gAwIBAgIUIYdQv74LF6tTl3VXgrlOtmP4ISIwDQYJKoZIhvcNAQEL
BQAwazELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcM
DVNhbiBGcmFuY2lzY28xFzAVBgNVBAoMDlByaXZhdGVDQSBJbmMuMRYwFAYDVQQD
DA1NeSBQcml2YXRlIENBMB4XDTI1MDMxNTE4MzMxNVoXDTM1MDMxMzE4MzMxNVow
azELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNh
biBGcmFuY2lzY28xFzAVBgNVBAoMDlByaXZhdGVDQSBJbmMuMRYwFAYDVQQDDA1N
eSBQcml2YXRlIENBMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA334N
ELHlK/HeaG/lIu7cnsxKEmJO21QPzrMkLPiMn3dIDwEs4bAeD36la69YdsZ+Siii
A3thiv+NG+prLtgRyn+zIvaTlBi+DZ8ItyCbKaYxMHx4LIjW0LE3T91UufnK1cCJ
6Qir6Yk18q/sYmoaOQvXVrC4y08dqpSJBYDTc9VSPUFHHeyL/DWdAIRpUcRYIZhE
a7wp7p3LOfuBkYfrSd2uj5feEcr26ghQvzraz1pTexhgrmqA6Onu1FN2YeUP+RkP
Oig0DLs3G0yYF3gLdyorwyVkYW5eMSD3DTM7ogYUFt2AZ6rBbIYQm9JmOtKZT9Wh
8axzQj6vWCGp6mDc84cOkZFlIpkyhUABVcXEdIWvWZmrMFJojYbYLYuVn/KIjoZW
C23x4DvvCxWF52FkyQ4o4bECX7/C888f/DhJYHQ+ZiGZkcZTygqiF+9gK3cq04Kf
/y0LvjY09XZp5SGanSmTTnoKIl2o/v9fNr/rQn1BrJgUoUF4zGG95C3Jv3VFOQxP
XcJ7LDmYcXS3LFt3v/rF8m6kNIkivX4xKVI1buHjRdhm/maJkI7rUqUXVMq/fCmI
31d4wdScSsiKcIthXGdDUn0WqRyc3w4QI+H6lTgk+mVAaBoWzB+lKqf89jatKS2J
nVzglbKFOmQTLibojsaNLHLYN8vIE0dDnVAaT3cCAwEAAaNTMFEwHQYDVR0OBBYE
FP/gxFHW1XFWYSSPphU/Oymj30PWMB8GA1UdIwQYMBaAFP/gxFHW1XFWYSSPphU/
Oymj30PWMA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggIBADDcQpxJ
VPWWlIFEXW1SXbq3Cy1w6NBuc7hmGhBB7FJVcbIpkuz6PiWyXBneHFAf0g0kB68g
NEpoiLVtbnA1VzMGLffex1fWrZRDV8I/70hNKc+p4vvbaOHusMMkGGgNjiPIV6k8
3J6eL+X72nsY8TwP+W4OzrQH+H4xTOZr+5tZYSa28a6pa56m983zvhQpfivIsS8g
G0Jz7ixS/cdUEnrdvvcTHdk5QWZYx2NFW48/Uzp7u6eFnRf7cpC7UzdsYARp0/2M
P1v8qLkSreXqky860wYimd2WhFSvJ9n55w0jdaksIXzGJ5oy58Bht+80c/cvac32
O2unLOsgIAFyfIymTzAf/1Vu1w63Ls0py80/Vz/dse8sSAaHw4PF3UynWZprxzVl
a2pU8O0hpxlRRnk8UrVPgFqV51qifnHun8tz0aJi6rlq7sUusouw7OZUdblrDGDe
yuNz+YfuKIxrPE0KYYROEjsJHXHNVuFESaJT++LyfGcbRvouSsjHCdUaLEkGpFHD
DUQIoqXl4rlrvAaB3jn1P9Wh7uER/8+N9AUCk/cTv9rh15l8gogtNeWgHkeyTLIs
xcu+6WRAnK9eu0vSM7zMM3y8b0pph1UhgbQTow3NajP4u5HVoIzKjfD/Mc2pMIKR
gNzJgq2rBh+ytZgv31JGEcG/DwfPrC7eANAy
-----END CERTIFICATE-----
`

	t.Run("should load custom CA from string", func(t *testing.T) {
		tlsCfg, err := getCACertTLSCfg(caCert, "")
		require.NoError(t, err)
		require.NotNil(t, tlsCfg)
		require.NotNil(t, tlsCfg.RootCAs)
	})

	t.Run("should load custom CA from file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "ca-cert-*.pem")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(caCert))
		require.NoError(t, err)
		tmpFile.Close()

		tlsCfg, err := getCACertTLSCfg("", tmpFile.Name())
		require.NoError(t, err)
		require.NotNil(t, tlsCfg)
		require.NotNil(t, tlsCfg.RootCAs)
	})

	t.Run("should return nil if no CA is provided", func(t *testing.T) {
		tlsCfg, err := getCACertTLSCfg("", "")
		require.NoError(t, err)

		systemCertPool, err := x509.SystemCertPool()
		require.NoError(t, err)

		systemTlsCfg := &tls.Config{
			RootCAs:    systemCertPool,
			MinVersion: tls.VersionTLS12,
		}

		require.Equal(t, len(systemTlsCfg.Certificates), len(tlsCfg.Certificates))
	})
}
