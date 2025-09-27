package uifs_test

import (
	"embed"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	uifs "github.com/roboslone/go-ui-fs"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata
	testFS embed.FS
	//go:embed testdata/response.txt
	expectedResponse []byte
	//go:embed testdata/index.html
	expectedIndex []byte
)

func getServer(t *testing.T) (string, func()) {
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server := &http.Server{Handler: uifs.Handler(testFS, uifs.WithPrefix("testdata"))}

	go func() {
		err := server.Serve(l)
		require.EqualError(t, err, http.ErrServerClosed.Error())
	}()

	time.Sleep(100 * time.Millisecond)

	return fmt.Sprintf("http://%s", l.Addr().String()), func() {
		require.NoError(t, server.Close())
	}
}

func TestUIFS(t *testing.T) {
	t.Run("sanity check", func(t *testing.T) {
		for _, name := range []string{"response.txt", "index.html"} {
			f, err := testFS.Open(filepath.Join("testdata", name))
			require.NoError(t, err, name)
			require.NoError(t, f.Close(), name)
		}
	})

	t.Run("server", func(t *testing.T) {
		baseURL, close := getServer(t)
		defer close()

		for range 3 { // repeated reads
			for _, url := range []string{
				"",                    // directory read
				"response.txt",        // file read
				"path/does/not/exist", // fallback to index.html
			} {
				url = fmt.Sprintf("%s/%s", baseURL, url)

				response, err := http.Get(url)
				require.NoError(t, err)

				body, err := io.ReadAll(response.Body)
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())

				require.Equal(t, http.StatusOK, response.StatusCode, "url: %q, body:\n%s", url, body)
			}
		}

		// ensure correct asset content
		response, err := http.Get(fmt.Sprintf("%s/response.txt", baseURL))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		content, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, expectedResponse, content)

		// ensure correct fallback content
		response, err = http.Get(fmt.Sprintf("%s/path/does/not/exist", baseURL))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		content, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, expectedIndex, content)
	})
}
