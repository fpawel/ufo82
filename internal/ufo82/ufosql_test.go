package ufo82

import (
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"math"
	"math/rand"
	"testing"
	"time"
)

func (x DB) addNewParty(t time.Time) PartyID {
	r, err := x.Conn.Exec(`INSERT INTO parties (created_at) VALUES ($1);SELECT last_insert_rowid();`, t)
	if err != nil {
		panic(err)
	}
	partyID, err := r.LastInsertId()
	if err != nil {
		panic(err)
	}
	return PartyID(partyID)
}

func (x DB) addNewProduct(partyID PartyID, order, serial int64) ProductID {
	r, err := x.Conn.Exec(`
INSERT INTO products (party_id, order_in_party, ProductNumber) 
VALUES ($1, $2, $3);
SELECT last_insert_rowid();`, partyID, order, serial)
	if err != nil {
		panic(err)
	}
	productID, err := r.LastInsertId()
	if err != nil {
		panic(err)
	}
	return ProductID(productID)
}

func TestFolderPath(t *testing.T) {
	fmt.Println(ProductName.Path())
}

func TestCreateTestDB(t *testing.T) {
	fmt.Println(ProductName.Path())
	db := MustConnectDB("products.db")
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	tm := func(y, m, d int) time.Time {
		return time.Date(y, time.Month(m), d, 8, 0, 0, 0, time.Local)
	}

	for _, t := range []time.Time{
		tm(2015, 2, 2),
		tm(2015, 2, 3),
		tm(2015, 2, 4),

		tm(2016, 3, 8),
		tm(2016, 3, 9),
		tm(2016, 3, 10),
		tm(2016, 3, 11),

		tm(2017, 7, 11),
		tm(2017, 7, 12),
		tm(2017, 7, 13),
		tm(2017, 7, 14),
		tm(2017, 7, 15),
		tm(2017, 7, 16),
	} {
		partyID := db.addNewParty(t)
		for n := int64(0); n < 10; n++ {
			productID := db.addNewProduct(partyID, n, int64(partyID)+n)
			var xs []Sensitivity
			f1 := newFunc(rnd)
			f2 := newFunc(rnd)
			for nPt := int64(0); nPt < 1000; nPt++ {
				x := float64(nPt)
				xs = append(xs, Sensitivity{
					StoredAt: t.Add(time.Second * time.Duration(nPt)),
					Value:    f1(x) - f2(x),
				})
			}
			db.addSensitivities(productID, xs)
		}
	}

	if err := db.Conn.Close(); err != nil {
		panic(err)
	}
}

func TestYears(t *testing.T) {
	db := MustConnectDB("products.db")
	fmt.Printf("%+v", db.GetYears())
	if err := db.Conn.Close(); err != nil {
		panic(err)
	}
}

func TestSensitivities(t *testing.T) {
	db := MustConnectDB("products.db")
	fmt.Printf("%+v", db.GetSensitivitiesByProductID(1))
	if err := db.Conn.Close(); err != nil {
		panic(err)
	}
}

func TestProducts(t *testing.T) {
	db := MustConnectDB("products.db")
	var xs []Product
	err := db.Conn.Select(&xs, `SELECT * FROM products LIMIT 10;`)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("%+v", xs)
	if err := db.Conn.Close(); err != nil {
		panic(err)
	}
}

func (x *DB) addSensitivities(productID ProductID, sensitivities []Sensitivity) {
	sql := "INSERT INTO sensitivities (product_id, stored_at, value) VALUES "

	for n, s := range sensitivities {
		sql += fmt.Sprintf("(%d,'%s',%v)", productID,
			s.StoredAt.UTC().Format(time.RFC3339), s.Value)
		if n == len(sensitivities)-1 {
			sql += ";"
		} else {
			sql += ", "
		}
	}

	t := time.Now()
	_, err := x.Conn.Exec(sql)
	fmt.Println("+", productID, time.Since(t))
	if err != nil {
		panic(err)
	}
}

func newFunc(rnd *rand.Rand) func(float64) float64 {

	return func(x float64) (r float64) {
		for i := float64(0); i < 5; i++ {
			r += rnd.Float64() * 10 * math.Sin(x*i)
		}
		return
	}

}
