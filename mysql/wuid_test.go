package wuid

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edwingeng/wuid/internal"
	_ "github.com/go-sql-driver/mysql" // mysql driver
)

type simpleLogger struct{}

func (this *simpleLogger) Info(args ...interface{}) {}
func (this *simpleLogger) Warn(args ...interface{}) {}

var sl = &simpleLogger{}

func init() {
	// Create test table
	addr, user, pass, dbName, table := getMysqlConfig()

	var dsn string
	dsn += user
	if len(pass) > 0 {
		dsn += ":" + pass
	}
	dsn += "@tcp(" + addr + ")/" + dbName

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("mysql connection error: ", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE ` + fmt.Sprintf("%s.%s", dbName, table) + ` (
		h int(10) NOT NULL AUTO_INCREMENT,
		x tinyint(4) NOT NULL DEFAULT '0',
		PRIMARY KEY (x),
		UNIQUE KEY h (h)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8;`)
	if err != nil {
		fmt.Println("Cannot create table error: ", err)
	}
}

func getMysqlConfig() (string, string, string, string, string) {
	return "127.0.0.1:3306", "root", "", "test", "wuid"
}

func TestWUID_LoadH24FromMysql(t *testing.T) {
	var nextValue uint64
	g := NewWUID("default", sl)
	for i := 0; i < 1000; i++ {
		err := g.LoadH24FromMysql(getMysqlConfig())
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			nextValue = atomic.LoadUint64(&g.w.N)
		} else {
			nextValue = ((nextValue >> 40) + 1) << 40
		}
		if atomic.LoadUint64(&g.w.N) != nextValue {
			t.Fatalf("g.w.N is %d, while it should be %d. i: %d", atomic.LoadUint64(&g.w.N), nextValue, i)
		}
		for j := 0; j < rand.Intn(10); j++ {
			g.Next()
		}
	}
}

func TestWUID_LoadH24FromMysql_Error(t *testing.T) {
	g := NewWUID("default", sl)
	addr, user, pass, dbName, table := getMysqlConfig()

	if g.LoadH24FromMysql("", user, pass, dbName, table) == nil {
		t.Fatal("addr is not properly checked")
	}
	if g.LoadH24FromMysql(addr, "", pass, dbName, table) == nil {
		t.Fatal("user is not properly checked")
	}
	if g.LoadH24FromMysql(addr, user, pass, "", table) == nil {
		t.Fatal("dbName is not properly checked")
	}
	if g.LoadH24FromMysql(addr, user, pass, dbName, "") == nil {
		t.Fatal("table is not properly checked")
	}

	if err := g.LoadH24FromMysql("127.0.0.1:30000", user, pass, dbName, table); err == nil {
		t.Fatal("LoadH24FromMysql should fail when is address is invalid")
	}
}

func TestWUID_LoadH24FromMysql_UserPass(t *testing.T) {
	var err error
	g := NewWUID("default", sl)
	addr, _, _, dbName, table := getMysqlConfig()
	err = g.LoadH24FromMysql(addr, "wuid", "abc123", dbName, table)
	if err != nil {
		if strings.Contains(err.Error(), "Access denied for user") {
			t.Log("you need to create a user in your MySQL. username: wuid, password: abc123")
		} else {
			t.Fatal(err)
		}
	}
	err = g.LoadH24FromMysql(addr, "wuid", "nopass", dbName, table)
	if err == nil {
		t.Fatal("LoadH24FromMysql should fail when the password is incorrect")
	}
}

func TestWUID_Next_Renew(t *testing.T) {
	g := NewWUID("default", sl)
	err := g.LoadH24FromMysql(getMysqlConfig())
	if err != nil {
		t.Fatal(err)
	}

	n1 := g.Next()
	kk := ((internal.CriticalValue + internal.RenewInterval) & ^internal.RenewInterval) - 1

	g.w.Reset((n1 >> 40 << 40) | kk)
	g.Next()
	time.Sleep(time.Millisecond * 200)
	n2 := g.Next()

	g.w.Reset((n2 >> 40 << 40) | kk)
	g.Next()
	time.Sleep(time.Millisecond * 200)
	n3 := g.Next()

	if n2>>40 == n1>>40 || n3>>40 == n2>>40 {
		t.Fatalf("the renew mechanism does not work as expected: %x, %x, %x", n1>>40, n2>>40, n3>>40)
	}
}

func TestWithSection(t *testing.T) {
	g := NewWUID("default", sl, WithSection(15))
	err := g.LoadH24FromMysql(getMysqlConfig())
	if err != nil {
		t.Fatal(err)
	}
	if g.Next()>>60 != 15 {
		t.Fatal("WithSection does not work as expected")
	}
}

func Example() {
	// Setup
	g := NewWUID("default", nil)
	_ = g.LoadH24FromMysql("127.0.0.1:3306", "root", "", "test", "wuid")

	// Generate
	for i := 0; i < 10; i++ {
		fmt.Printf("%#016x\n", g.Next())
	}
}

func BenchmarkLoadH24FromMysql(b *testing.B) {
	// Setup
	g := NewWUID("default", nil)
	err := g.LoadH24FromMysql(getMysqlConfig())
	if err != nil {
		b.Fatal(err)
	}

	// Generate
	for n := 0; n < b.N; n++ {
		g.Next()
	}

	fmt.Println(" - " + b.Name() + " complete - ")
}
