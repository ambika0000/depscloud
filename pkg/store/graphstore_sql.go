package store

import (
	"database/sql"
	"fmt"
)

const createGraphDataTable = `CREATE TABLE IF NOT EXISTS dts_graph (
 	graph_data_type VARCHAR(55),
	k1 CHAR(64),
	k2 CHAR(64),
	version BIGINT,
	encoding TINYINT,
	graph_data TEXT,
	PRIMARY KEY (graph_data_type, k1, k2),
	UNIQUE (graph_data_type, k1, version)
);`

const insertGraphData = `INSERT INTO dts_graph 
	(graph_data_type, k1, k2, version, encoding, graph_data) 
	VALUES (?, ?, ?, ?, ?, ?);`

const selectGraphDataPrimaryKey = `SELECT * 
FROM dts_graph 
WHERE graph_data_type = ? and k1 = ? and k2 = ?;`

const selectGraphDataSecondaryKey = `SELECT * 
FROM dts_graph
WHERE graph_data_type = ? and k1 = ? and version = ?;`

const selectGraphDataUpstreamDependencies = `SELECT *
FROM dts_graph 
WHERE k1 IN (SELECT k2 FROM dts_graph WHERE k1 = ? and k1 != k2)
AND k1 == k2;`

const selectGraphDataDownstreamDependencies = `SELECT *
FROM dts_graph
WHERE k2 IN (SELECT k1 FROM dts_graph WHERE k2 = ? and k1 != k2)
AND k1 == k2;`

// NewSQLGraphStore constructs a new GraphStore with a sql driven backend. Current
// queries support sqlite3 but should be able to work on mysql as well.
func NewSQLGraphStore(db *sql.DB) (GraphStore, error) {
	if _, err := db.Exec(createGraphDataTable); err != nil {
		return nil, err
	}

	return &sqlGraphStore{
		db: db,
	}, nil
}

var _ GraphStore = &sqlGraphStore{}

type sqlGraphStore struct {
	db *sql.DB
}

func (gs *sqlGraphStore) Put(item *GraphItem) error {
	_, err := gs.db.Exec(insertGraphData,
		item.GraphItemType, string(item.K1), string(item.K2),
		item.Version, item.Encoding, string(item.GraphItemData))

	return err
}

func (gs *sqlGraphStore) FindByPrimary(key *PrimaryKey) (*GraphItem, error) {
	rows, err := gs.db.Query(selectGraphDataPrimaryKey, key.GraphItemType, string(key.K1), string(key.K2))
	if err != nil {
		return nil, err
	}

	res, err := readFullyAndClose(rows)
	if err != nil {
		return nil, err
	}

	if len(res) != 1 {
		return nil, fmt.Errorf("failed to read record using primary key: %v", key)
	}

	return res[0], nil
}

func (gs *sqlGraphStore) FindBySecondary(key *SecondaryKey) (*GraphItem, error) {
	rows, err := gs.db.Query(selectGraphDataSecondaryKey, key.GraphItemType, string(key.K1), key.Version)
	if err != nil {
		return nil, err
	}

	res, err := readFullyAndClose(rows)
	if err != nil {
		return nil, err
	}

	if len(res) != 1 {
		return nil, fmt.Errorf("failed to read record using secondary key: %v", key)
	}

	return res[0], nil
}

func (gs *sqlGraphStore) FindUpstream(key []byte) ([]*GraphItem, error) {
	rows, err := gs.db.Query(selectGraphDataUpstreamDependencies, string(key))
	if err != nil {
		return nil, err
	}

	res, err := readFullyAndClose(rows)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (gs *sqlGraphStore) FindDownstream(key []byte) ([]*GraphItem, error) {
	rows, err := gs.db.Query(selectGraphDataDownstreamDependencies, string(key))
	if err != nil {
		return nil, err
	}

	res, err := readFullyAndClose(rows)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func readFullyAndClose(rows *sql.Rows) ([]*GraphItem, error) {
	defer rows.Close()

	results := make([]*GraphItem, 0)

	for ; rows.Next() ; {
		var (
			graphItemType string
			k1 string
			k2 string
			version uint64
			encoding uint8
			graphItemData string
		)

		if err := rows.Scan(&graphItemType, &k1, &k2, &version, &encoding, &graphItemData); err != nil {
			return nil, err
		}

		results = append(results, &GraphItem{
			GraphItemType: graphItemType,
			K1: []byte(k1),
			K2: []byte(k2),
			Version: version,
			Encoding: encoding,
			GraphItemData: []byte(graphItemData),
		})
	}

	return results, nil
}
