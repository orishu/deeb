package sqlite

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/orishu/deeb/internal/lib"
	"github.com/stretchr/testify/require"
)

func Test_basic_sqlite_access(t *testing.T) {
	dbFile, err := ioutil.TempFile("", fmt.Sprintf("%s-*.sqlite", t.Name()))
	defer os.Remove(dbFile.Name())

	require.NoError(t, err)
	b, err := New(dbFile.Name(), lib.NewDevelopmentLogger())
	require.NoError(t, err)
	ctx := context.Background()
	err = b.Start(ctx)
	require.NoError(t, err)
	defer b.Stop(ctx)

	be := b.(*Backend)
	row := be.db.QueryRow("SELECT 1+1")
	var data int64
	err = row.Scan(&data)
	require.NoError(t, err)
	require.Equal(t, int64(2), data)
}
