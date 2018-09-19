package ufo82

import (
	"fmt"
	"github.com/jmoiron/sqlx"
)

type DB struct {
	Conn *sqlx.DB
}

func (x DB) Close() error {
	return x.Conn.Close()
}

func MustConnectDB(filename string) (x DB) {

	var createdNewFile bool
	x.Conn = sqlx.MustConnect("sqlite3", filename)
	x.Conn.MustExec(intiDBSQL)
	if createdNewFile {
		x.Conn.MustExec(createDBSQL)
	}
	return
}

func (x DB) GetLastPartyID() (r PartyID) {
	err := x.Conn.Get(&r, `SELECT party_id FROM parties ORDER BY created_at DESC LIMIT 1;`)
	if err != nil {
		panic(err)
	}
	return
}

func (x DB) GetLastPartyProducts() (products []Product) {
	err := x.Conn.Select(&products, `
SELECT * FROM products 
WHERE party_id = ( SELECT party_id FROM parties ORDER BY created_at DESC LIMIT 1) 
ORDER BY order_in_party ASC;`)
	if err != nil {
		panic(err)
	}
	return
}

func (x DB) GetYears() (xs []int) {
	err := x.Conn.Select(&xs, `
SELECT cast(strftime('%Y', created_at) AS INT) AS year FROM parties GROUP BY year;`)
	if err != nil {
		panic(err)
	}
	return
}

func (x DB) GetDaysOfYearMonth(ym YearMonth) (xs []int64) {
	err := x.Conn.Select(&xs, `
SELECT cast( strftime('%d', created_at) AS INT) AS day FROM parties
WHERE  cast(strftime('%Y', created_at) AS INT) = $1 AND cast(strftime('%m', created_at) AS INT) = $2
GROUP BY day;
`, ym.Year, ym.Month)
	if err != nil {
		panic(err)
	}
	return
}

func (x DB) GetMonthsOfYear(year int) (xs []int) {
	err := x.Conn.Select(&xs, `
SELECT cast( strftime('%m', created_at) AS INT) AS month FROM parties
WHERE cast(strftime('%Y', created_at) AS INT) = $1
GROUP BY month;
`, year)
	if err != nil {
		panic(err)
	}
	return
}

func (x DB) GetPartiesOfYearMonthDay(ym YearMonthDay) (xs []Party) {
	err := x.Conn.Select(&xs, `
SELECT * FROM parties
WHERE
  cast(strftime('%Y', created_at) AS INT) = $1 AND
  cast(strftime('%m', created_at) AS INT) = $2 AND
  cast(strftime('%d', created_at) AS INT) = $3
ORDER BY created_at;
`, ym.Year, ym.Month, ym.Day)
	if err != nil {
		panic(err)
	}
	return
}

func (x DB) GetPartyByID(partyID PartyID) (party Party, products []Product) {
	err := x.Conn.Get(&party, `SELECT * FROM parties WHERE party_id = $1;`, partyID)
	if err != nil {
		panic(err)
	}
	err = x.Conn.Select(&products, `SELECT * FROM products WHERE party_id = $1 ORDER BY order_in_party ASC;`, partyID)
	if err != nil {
		panic(err)
	}
	return
}

func (x DB) GetSensitivitiesByProductID(productID ProductID) (xs []Sensitivity) {
	err := x.Conn.Select(&xs, `
SELECT stored_at,value FROM sensitivities
WHERE product_id = $1
GROUP BY stored_at;
`, productID)
	if err != nil {
		panic(err)
	}
	return
}

func (x DB) AddNewSensitivity(productID ProductID, sensitivity float32) {
	_, err := x.Conn.Exec(`
INSERT INTO sensitivities (product_id, value) 
VALUES ($1,$2);`, productID, sensitivity)
	if err != nil {
		panic(err)
	}
}

func (x DB) ClearProductSensitivities(productID ProductID) {
	_, err := x.Conn.Exec(`DELETE FROM sensitivities WHERE product_id = $1;`, productID)
	if err != nil {
		panic(err)
	}
}

func (x DB) ApplyCurrentProductSerial(inp ProductOrderSerial) string {
	partyID := x.GetLastPartyID()
	_, products := x.GetPartyByID(partyID)
	strProduct := fmt.Sprintf("продукт №%d, заводской номер %d", inp.Order+1, inp.Serial)

	for _, p := range products {
		if p.ProductNumber == int64(inp.Serial) {
			if p.Order == int64(inp.Order) {
				return strProduct
			}
			return fmt.Sprintf("%s: дублирование заводского номера", strProduct)
		}
	}

	for _, p := range products {
		if p.Order == int64(inp.Order) {
			if inp.Serial <= 0 {
				if _, err := x.Conn.Exec(`DELETE FROM products WHERE product_id = $1`, p.ProductID); err != nil {
					panic(err)
				}
				return fmt.Sprintf("Удалён продукт №%d", inp.Order+1)
			} else {
				if _, err := x.Conn.Exec(`UPDATE products SET product_number = $1 WHERE product_id = $2`,
					inp.Serial, p.ProductID); err != nil {
					panic(err)
				}
				return fmt.Sprintf("Изменён %s", strProduct)
			}

		}
	}
	_, err := x.Conn.Exec(`
INSERT INTO products (party_id, product_number, order_in_party)  
VALUES ($1, $2, $3);`, partyID, inp.Serial, inp.Order)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("Добавлен в текущую партию %s", strProduct)
}

func (x DB) CreateNewParty() {

	_, err := x.Conn.Exec(`
DELETE FROM parties
WHERE NOT exists
( SELECT sensitivities.product_id
  FROM sensitivities
    INNER JOIN products on sensitivities.product_id = products.product_id
  WHERE products.party_id = parties.party_id);
DELETE FROM products
WHERE
  party_id=(SELECT parties.party_id FROM parties ORDER BY created_at DESC LIMIT 1) AND
  ( NOT exists(
      SELECT sensitivities.product_id
      FROM sensitivities
      WHERE sensitivities.product_id = products.product_id)
  );`)
	if err != nil {
		panic(err)
	}

	if len(x.GetYears()) == 0 {
		_, err := x.Conn.Exec(`
INSERT INTO parties DEFAULT VALUES;
INSERT INTO products (party_id, product_number, order_in_party)  VALUES (last_insert_rowid(), 1, 0);`)
		if err != nil {
			panic(err)
		}
	}

	_, products := x.GetPartyByID(x.GetLastPartyID())

	r, err := x.Conn.Exec(`INSERT INTO parties DEFAULT VALUES; SELECT last_insert_rowid()`)
	if err != nil {
		panic(err)
	}
	v, err := r.LastInsertId()
	if err != nil {
		panic(err)
	}
	newPartyID := PartyID(v)
	for _, p := range products {
		_, err := x.Conn.Exec(`
INSERT INTO products (party_id, product_number, order_in_party)  
VALUES ($1, $2, $3);`, newPartyID, p.ProductNumber, p.Order)
		if err != nil {
			panic(err)
		}
	}

	return
}

const intiDBSQL = `
PRAGMA foreign_keys = ON;
PRAGMA encoding = 'UTF-8';
`

const createDBSQL = `
CREATE TABLE parties (
  party_id        INTEGER PRIMARY KEY,
  created_at      TIMESTAMP NOT NULL DEFAULT current_timestamp
);

INSERT INTO parties DEFAULT VALUES;


CREATE TABLE products (
  product_id INTEGER PRIMARY KEY,
  party_id INTEGER NOT NULL,
  product_number INTEGER NOT NULL,
  order_in_party INTEGER NOT NULL,
  CONSTRAINT unique_order_in_party UNIQUE (party_id, order_in_party),
  CONSTRAINT unique_product_number_in_party UNIQUE (party_id, product_number),
  CONSTRAINT positive_product_number CHECK (product_number > 0),
  CONSTRAINT not_negative_order_in_party CHECK (order_in_party > 0 OR order_in_party = 0),
  FOREIGN KEY(party_id) REFERENCES parties(party_id) ON DELETE CASCADE
);

INSERT INTO products (party_id, product_number, order_in_party)  VALUES (last_insert_rowid(), 1, 0);

CREATE TABLE sensitivities (
  product_id INTEGER NOT NULL,
  stored_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
  value REAL NOT NULL,
  FOREIGN KEY(product_id) REFERENCES products(product_id) ON DELETE CASCADE
);
`
