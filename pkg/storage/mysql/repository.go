package mysql

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"

	"kopsvas19p.interdigital.com/robitzsx/ardent/pkg/domain/models"
)

type Storage struct {
	db *sql.DB
}

type foreignKey struct {
	col       string
	refTblIdx int
	refTblCol string
}

type table struct {
	Name string
	Cols []string

	// Primary keys for this table. Mapped from DB.
	PK string

	// Foreign keys for this table. Mapped from DB.
	FK []foreignKey

	ColNames string
	ValSubs  string
}

type insert struct {
	Entities interface{}

	Stmt *string
	Vals *[]interface{}

	lastInsertId int64
	affectedRows int64
}

type tblRelation struct {
	tblIdx int
	fkIdx  int
}

type qInfo struct {
	entityFieldIdx int

	tblToUpdateIdx int
	tblToQueryIdx  int

	relations *[]tblRelation

	qStr string
}

type entity struct {
	tblIdx int
}

type sqlQuery interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type Read interface {
	Reader(obj interface{}) (interface{}, error)
}

const (
	addStmt = "INSERT INTO %s%s VALUES %s"
)

const (
	computeTblIdx = iota
	networkTblIdx
	subnetTblIdx
	infraServiceTblIdx
	securityGroupTblIdx
	configTblIdx
	flavorsTblIdx
	securityGrpRulesTblIdx

	// Add a new index above this line for any new infra entity in models.

	lastMappedTblIdx // last Mapped Table + 1

	// Tables existing only in DB, but not mapped to entities. They represent
	// bridge tables to define Many-to-Many relations.
	computeNetworkTblIdx = iota - 1

	// Add a new index for bridge table (defining relation between two Infra
	// tables) below.

	lastTblIdx
)

// Reflects tables in DB.
var tables []table

// Initialize it with Names of Infra entities.
var infraEntity []string

// This map will be initialized with pointer to structures corresponding to
// tables existing in DB.
var infraTblIdxMap map[string]int

// Contains mapping from infrastructure properties to columns in the DB tables.
var infraPropTblColMap map[string]map[string]string

// logger to be used for logging.
var logger *logrus.Logger

func initTables() {
	// Assumptions for DB Schema:
	//		1. Tables are related using FKs, and FKs used in relation are referencing PKs.

	// This array of tables reflects tables in MySQL DB. These table are being hard-coded here,
	// but its field can be mapped directly from DB by reading schema.
	// Tables corresponding to Infra models are populated first and they are being populated
	// as per *TblIdx constants.
	tables = []table{
		{
			"`compute-nodes`",
			[]string{"`idcompute-nodes`", "`availability-zone`", "`name`", "`tier-level`", "`vcpus`", "`ram`", "`disk`"},
			"`idcompute-nodes`",
			nil,
			"",
			"",
		},
		{
			"`networks`",
			[]string{"`idnetworks`", "`os-identifier`", "`category`"},
			"`idnetworks`",
			nil,
			"",
			"",
		},
		{
			"`subnets`",
			[]string{"`idsubnets`", "`os-identifier`", "`category`"},
			"`idsubnets`",
			//[]foreignKey{
			//	{`idsubnets`, networkTblIdx, "`idnetworks`"},
			//},
			nil,
			"",
			"",
		},
		{
			"`infrastructure-services`",
			[]string{"`idinfrastructure-services`", "`service-type`", "`value`"},
			"`idinfrastructure-services`",
			nil,
			"",
			"",
		},
		{
			"`security-groups`",
			[]string{"`idsecurity-groups`", "`os-identifier`", "`category`"},
			"`idsecurity-groups`",
			nil,
			"",
			"",
		},
		{
			"`config`",
			[]string{"`conf-key`", "`value`"},
			"",
			nil,
			"",
			"",
		},
		{
			"`flavors`",
			[]string{"`name`", "`vcpus`", "`ram`", "`disk`"},
			"",
			nil,
			"",
			"",
		},
		{
			"`security-group-rules`",
			[]string{"`name`", "`protocol`", "`port`"},
			"",
			nil,
			"",
			"",
		},
		// All DB tables corresponding to Infra models are added above.

		// All bridge tables (defining relation between two Infra tables) are added below.
		{
			"`compute-nodes-networks`",
			[]string{"`idcompute-nodes`", "`idnetworks`"},
			"", // Primary key in these relation tables is not of significance.
			[]foreignKey{
				{"`idcompute-nodes`", computeTblIdx, "`idcompute-nodes`"},
				{"`idnetworks`", networkTblIdx, "`idnetworks`"},
			},
			"",
			"",
		},
	}
	for i, _ := range tables {
		logger.Debugf("Primary Key for table: %s is: %s", tables[i].Name, tables[i].PK)
		logger.Debugf("Foreign Key(s) for table: %s is/are: %v", tables[i].Name, tables[i].FK)

		tables[i].ColNames += tables[i].genColNamesStr()
		tables[i].ValSubs += tables[i].genValSubsStr()
	}
}

func initInfraEntities() {
	// Make change if there is any modification in infrastructure entity
	// names in infra models.
	infraEntity = make([]string, lastMappedTblIdx)
	infraEntity[computeTblIdx] = "Compute"
	infraEntity[networkTblIdx] = "Network"
	infraEntity[subnetTblIdx] = "Subnet"
	infraEntity[infraServiceTblIdx] = "InfraService"
	infraEntity[securityGroupTblIdx] = "SecurityGroup"
	infraEntity[configTblIdx] = "Config"
	infraEntity[flavorsTblIdx] = "Flavor"
	infraEntity[securityGrpRulesTblIdx] = "SecurityGrpRule"
}

func initInfraToTablesMap() {
	// Initialize Infra structures to DB table structures mapping.
	// Currently, it is hardcoded.
	infraTblIdxMap = make(map[string]int, lastMappedTblIdx)
	infraTblIdxMap[infraEntity[computeTblIdx]] = computeTblIdx
	infraTblIdxMap[infraEntity[networkTblIdx]] = networkTblIdx
	infraTblIdxMap[infraEntity[subnetTblIdx]] = subnetTblIdx
	infraTblIdxMap[infraEntity[infraServiceTblIdx]] = infraServiceTblIdx
	infraTblIdxMap[infraEntity[securityGroupTblIdx]] = securityGroupTblIdx
	infraTblIdxMap[infraEntity[configTblIdx]] = configTblIdx
	infraTblIdxMap[infraEntity[flavorsTblIdx]] = flavorsTblIdx
	infraTblIdxMap[infraEntity[securityGrpRulesTblIdx]] = securityGrpRulesTblIdx

	logger.Debugf("infraTblIdxMap: %v", infraTblIdxMap)

	// Make change if there is any change in the infra model or DB col name.
	infraPropTblColMap = make(map[string]map[string]string, lastMappedTblIdx)

	infraPropTblColMap[infraEntity[computeTblIdx]] = make(map[string]string)
	infraPropTblColMap[infraEntity[computeTblIdx]]["AvailZone"] = "`availability-zone`"
	infraPropTblColMap[infraEntity[computeTblIdx]]["Name"] = "`name`"
	infraPropTblColMap[infraEntity[computeTblIdx]]["Tier"] = "`tier-level`"
	infraPropTblColMap[infraEntity[computeTblIdx]]["Vcpus"] = "`vcpus`"
	infraPropTblColMap[infraEntity[computeTblIdx]]["RAM"] = "`ram`"
	infraPropTblColMap[infraEntity[computeTblIdx]]["Disk"] = "`disk`"

	infraPropTblColMap[infraEntity[networkTblIdx]] = make(map[string]string)
	infraPropTblColMap[infraEntity[networkTblIdx]]["Identifier"] = "`os-identifier`"
	infraPropTblColMap[infraEntity[networkTblIdx]]["Category"] = "`category`"

	infraPropTblColMap[infraEntity[subnetTblIdx]] = make(map[string]string)
	infraPropTblColMap[infraEntity[subnetTblIdx]]["Identifier"] = "`os-identifier`"
	infraPropTblColMap[infraEntity[subnetTblIdx]]["Category"] = "`category`"

	infraPropTblColMap[infraEntity[infraServiceTblIdx]] = make(map[string]string)
	infraPropTblColMap[infraEntity[infraServiceTblIdx]]["ServiceType"] = "`service-type`"
	infraPropTblColMap[infraEntity[infraServiceTblIdx]]["Value"] = "`value`"

	infraPropTblColMap[infraEntity[securityGroupTblIdx]] = make(map[string]string)
	infraPropTblColMap[infraEntity[securityGroupTblIdx]]["Identifier"] = "`os-identifier`"
	infraPropTblColMap[infraEntity[securityGroupTblIdx]]["Category"] = "`category`"

	infraPropTblColMap[infraEntity[configTblIdx]] = make(map[string]string)
	infraPropTblColMap[infraEntity[configTblIdx]]["ConfKey"] = "`conf-key`"
	infraPropTblColMap[infraEntity[configTblIdx]]["Value"] = "`value`"

	infraPropTblColMap[infraEntity[flavorsTblIdx]] = make(map[string]string)
	infraPropTblColMap[infraEntity[flavorsTblIdx]]["Name"] = "`name`"
	infraPropTblColMap[infraEntity[flavorsTblIdx]]["Vcpus"] = "`vcpus`"
	infraPropTblColMap[infraEntity[flavorsTblIdx]]["RAM"] = "`ram`"
	infraPropTblColMap[infraEntity[flavorsTblIdx]]["Disk"] = "`disk`"

	infraPropTblColMap[infraEntity[securityGrpRulesTblIdx]] = make(map[string]string)
	infraPropTblColMap[infraEntity[securityGrpRulesTblIdx]]["Name"] = "`name`"
	infraPropTblColMap[infraEntity[securityGrpRulesTblIdx]]["Protocol"] = "`protocol`"
	infraPropTblColMap[infraEntity[securityGrpRulesTblIdx]]["Port"] = "`port`"

	logger.Debugf("infraPropTblColMap: %v", infraPropTblColMap)
}

func NewStorage(glogger *logrus.Logger, username string, password string, serverIp string, store string) (*Storage, error) {
	var err error

	logger = glogger
	logger.Debugf("mysql.Storage: NewStorage")

	s := new(Storage)

	dsn := username + ":" + password + "@tcp(" + serverIp + ":3306)/" + store
	s.db, err = sql.Open("mysql", dsn)
	if err != nil {
		logger.Errorf("sql.Open() error")
		return nil, err
	}

	err = s.db.Ping()
	if err != nil {
		logger.Errorf("db.Ping() error")
		return nil, err
	}

	initInfraEntities()
	initTables()
	initInfraToTablesMap()

	logger.Infof("NewStorage Initialized")

	return s, nil
}

func (s *Storage) Add(e []interface{}) error {
	logger.Debugf("mysql.Storage: Add")

	if e == nil {
		err := errors.New("Entity interface cannot be nil")
		logger.Errorf("%v", err)

		return err
	}

	insData := make([]insert, 0)

	for i, _ := range e {
		var tbl *table = nil

		vals := []interface{}{}
		var valSubsSet []string

		ename := reflect.TypeOf(e[i]).Elem().Name()
		tblIdx, okay := infraTblIdxMap[ename]
		if !okay {
			er := fmt.Sprintf("Corresponding table doesn't exist for entity: '%s'",
				ename)
			logger.Errorf("%s", er)

			return errors.New(er)
		}

		tbl = &tables[tblIdx]
		if len(tbl.FK) != 0 {
			logger.Debugf("Tbl: '%s' has Foreign Keys, these tables will be inserted in 2nd iteration",
				tbl.Name)
			continue
		}
		valSubs := tbl.ValSubs

		ei := reflect.ValueOf(e[i])
		logger.Debugf("Number of rows to be inserted in '%s' Table: %d",
			tbl.Name, ei.Len())

		for j := 0; j < ei.Len(); j++ {
			valSubsSet = append(valSubsSet, valSubs)

			prepareValsForInsert(tblIdx, ei.Index(j).Interface(), &vals)
		}
		valSubs = strings.Join(valSubsSet, ",")
		if len(vals) == 0 {
			logger.Warnf("No rows received for %s table", tbl.Name)
			continue
		}

		logger.Debugf("SQL Stmt prepared for inserting rows in %s table", tbl.Name)
		addStmt := fmt.Sprintf(addStmt, tbl.Name, tbl.ColNames, valSubs)
		logger.Debugf("Statement: %s", addStmt)
		logger.Debugf("Values: %v", vals)

		ins := insert{
			Stmt:     &addStmt,
			Vals:     &vals,
			Entities: e[i],
		}

		insData = append(insData, ins)
	}
	if err := s.txManage(insData); err != nil {
		logger.Errorf("Error returned by txManage()")
		return err
	}
	return nil
}

func prepareValsForInsert(tblIdx int, entity interface{}, vals *[]interface{}) {
	logger.Debugf("mysql.Storage: prepareValsForInsert")

	switch tblIdx {
	case computeTblIdx:
		e := entity.(models.Compute)
		logger.Debugf("Row to be inserted: %s   %s   %s   %d   %d   %d",
			e.AvailZone, e.Name, e.Tier, e.Vcpus, e.RAM, e.Disk)

		*vals = append(*vals, 0, e.AvailZone, e.Name, e.Tier,
			e.Vcpus, e.RAM, e.Disk)
	case networkTblIdx:
		e := entity.(models.Network)
		logger.Debugf("Row to be inserted: %s   %s", e.Identifier, e.Category)

		*vals = append(*vals, 0, e.Identifier, e.Category)
	case subnetTblIdx:
		e := entity.(models.Subnet)
		logger.Debugf("Row to be inserted: %s   %s", e.Identifier, e.Category)

		*vals = append(*vals, 0, e.Identifier, e.Category)
	case infraServiceTblIdx:
		e := entity.(models.InfraService)
		logger.Debugf("Row to be inserted: %s   %s", e.ServiceType, e.Value)

		*vals = append(*vals, 0, e.ServiceType, e.Value)
	case securityGroupTblIdx:
		e := entity.(models.SecurityGroup)
		logger.Debugf("Row to be inserted: %s   %s", e.Identifier, e.Category)

		*vals = append(*vals, 0, e.Identifier, e.Category)
	case configTblIdx:
		e := entity.(models.Config)
		logger.Debugf("Row to be inserted: %s   %s", e.ConfKey, e.Value)

		*vals = append(*vals, e.ConfKey, e.Value)
	case flavorsTblIdx:
		e := entity.(models.Flavor)
		logger.Debugf("Row to be inserted: %s   %d   %d   %d",
			e.Name, e.Vcpus, e.RAM, e.Disk)

		*vals = append(*vals, e.Name, e.Vcpus, e.RAM, e.Disk)
	}
}

func (s *Storage) Get(q *models.Query) (interface{}, error) {
	logger.Debugf("mysql.Storage: Get")

	if q.Entity == nil {
		er := "Nil Entity passed in Query"
		logger.Errorf("%s", er)

		return nil, errors.New(er)
	}

	// Get tables involved in Query.
	tblsMap, err := getTblSetForQ(q)
	if err != nil {
		return nil, err
	}
	var relations *[]tblRelation = nil
	var bridgeTblIdx *[]int = nil
	joinConds := ""
	if len(tblsMap) > 1 {
		// JOIN is required.
		logger.Debugf("More than one table involved in Query")

		relations, bridgeTblIdx, err = findTblRelations(tblsMap)
		if err != nil {
			logger.Errorf("Error in finding relations b/w tables")
			return nil, err
		}
		logger.Debugf("Relations: %v", *relations)

		joinConds = convertRelationsToCondStr(relations)

	}
	// Find query string.
	conditions := clause(q)
	logger.Debugf("conditions: %s", conditions)

	finalCond := ""
	if joinConds != "" {
		// JOIN constraints.
		finalCond += joinConds
	}
	logger.Debugf("joinConds: %s", finalCond)
	if conditions != "" {
		// Concatenate original conditions with JOIN constraints.
		if finalCond != "" {
			if strings.HasPrefix(conditions, "AND ") || strings.HasPrefix(conditions, "OR ") {
				finalCond += " "
			} else {
				finalCond += " AND "
			}
		}
		finalCond += conditions
	}

	tblName := tables[infraTblIdxMap[reflect.TypeOf(q.Entity).Name()]].Name

	// Form Query.
	queryStr := ""
	if finalCond == "" {
		// Query without any condition.
		queryStr = fmt.Sprintf("select %s.* from %s", tblName, tblName)
	} else {
		// Get all tables involved in query.
		tbls := []string{}
		for k, _ := range tblsMap {
			tbls = append(tbls, tables[infraTblIdxMap[k]].Name)
		}
		// Add additional bridge tables, if any.
		if bridgeTblIdx != nil {
			for _, v := range *bridgeTblIdx {
				tbls = append(tbls, tables[v].Name)
			}
		}
		tblStr := strings.Join(tbls, ",")
		queryStr = fmt.Sprintf("select %s.* from %s where %s", tblName, tblStr, finalCond)
	}
	logger.Debugf("SQL Query: %s", queryStr)

	logger.Debugf("Executing SQL Query")

	tblIdx, _ := infraTblIdxMap[reflect.TypeOf(q.Entity).Name()]
	e := entity{tblIdx}

	result, _, err := executeQuery(queryStr, e, s.db)

	return result, err
}

func (s *Storage) Remove(es interface{}) error {
	logger.Debugf("mysql.Storage: Remove")

	if es == nil {
		err := errors.New("No Entity found to remove")
		logger.Errorf("%v", err)

		return err
	}

	// Get table involved in Deletion.
	entityName := reflect.TypeOf(es).Name()

	tblIdx, _ := infraTblIdxMap[entityName]
	tblName := tables[tblIdx].Name

	logger.Debugf("Entity Name: %s to be considered for deletion", entityName)
	logger.Debugf("Rows to be deleted from table: %s", tblName)

	//Form query.
	deleteStr := ""
	deleteStr = fmt.Sprintf("DELETE FROM %s", tblName)

	// Add condition.
	var op []string
	condition := ""
	createQueryCondsForEntity(es, &op)
	if len(op) != 0 {
		logger.Debugf("Condition Clause in Statement is not empty")

		condition = strings.Join(op, " ")
		logger.Debugf("Clause: %s", condition)

		deleteStr = deleteStr + " WHERE " + condition
	}

	logger.Debugf("SQL Query: %s", deleteStr)

	logger.Debugf("Executing SQL Query")
	err := dbExec(deleteStr, s.db)
	if err != nil {
		logger.Errorf("Deletion from DB unsuccessful")
		return err
	}

	return nil
}

func (s *Storage) txManage(insData []insert) error {
	logger.Debugf("mysql.Storage: txManage")

	tx, err := s.db.Begin()
	if err != nil {
		logger.Errorf("Error returned by db.Begin(): %v", err)
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			logger.Errorf("Panic, so rolling back Tx and throwing panic")
			tx.Rollback()
			panic(p)
		} else if err != nil {
			// error, rollback
			logger.Errorf("Error, so rolling back Tx")
			tx.Rollback()
		} else {
			// commit
			logger.Debugf("Committing Tx")
			if err = tx.Commit(); err != nil {
				logger.Errorf("Error in tx.Commit(): %v", err)
			}
		}
	}()

	// Insert data into DB for tables that can be inserted independently. i.e.
	// they don't have any of their key dependent on any other table.
	err = txExec(tx, insData)
	if err != nil {
		logger.Errorf("Error returned by txExec() - first pass: %v", err)
		return err
	}

	// Check if dependent and bridge tables need to be updated.
	var insDataN *[]insert

	logger.Debugf("Populate dependent and bridge tables, if required")
	insDataN, err = genSqlStmtsForRelatedTbls(tx, insData)
	if err != nil {
		logger.Errorf("Error returned by genSqlStmtsForRelatedTbls()")
		return err
	}

	// Insert data into DB for tables that can be dependent (having foreign key to
	// another tables) and bridge tables.
	if insDataN == nil {
		logger.Debugf("No dependent/bridge tables to be inserted")
		return nil
	}
	err = txExec(tx, *insDataN)
	if err != nil {
		logger.Errorf("Error returned by txExec() - second pass: %v", err)
		return err
	}

	return nil
}

func txExec(tx *sql.Tx, insData []insert) error {
	logger.Debugf("mysql.Storage: txExec")

	for idx, _ := range insData {
		var res sql.Result

		logger.Debugf("Executing SQL Stmt: %s with vals: %v",
			*insData[idx].Stmt, *insData[idx].Vals)

		res, err := tx.Exec(*insData[idx].Stmt, *insData[idx].Vals...)
		if err != nil {
			logger.Errorf("Error returned by tx.Exec(): %v", err)
			return err
		}
		insData[idx].lastInsertId, err = res.LastInsertId()
		if err != nil {
			logger.Errorf("Error returned by res.LastInsertId(): %v", err)
			return err
		}
		insData[idx].affectedRows, err = res.RowsAffected()
		if err != nil {
			logger.Errorf("Error returned by res.RowsAffected(): %v", err)
			return err
		}
		logger.Debugf("Last Insert ID = %d, Rows affected = %d", insData[idx].lastInsertId,
			insData[idx].affectedRows)
	}
	return nil
}

func genSqlStmtsForRelatedTbls(tx *sql.Tx, insData []insert) (*[]insert, error) {
	var res *[]insert = nil

	logger.Debugf("mysql.Storage: genSqlStmtsForRelatedTbls")

	insDataN := make([]insert, 0)

	// Loop around all entities types received in Add() interface
	for idx, _ := range insData {
		entity := insData[idx].Entities

		ename := reflect.TypeOf(entity).Elem().Name()

		// Check and get queries related info that will be used to prepare
		// queries.
		qInfos, err := formReqdQueriesForEntity(entity)
		if err != nil {
			logger.Errorf("Error reported by formReqdQueriesForEntity()")
			return nil, err
		}
		if qInfos == nil {
			logger.Debugf("No Further table update required for Entity: '%s'",
				ename)
			continue
		}
		// Generate INSERT statements for all elements of entity.
		err = genSqlStmtsPerElem(tx, &insData[idx], qInfos, &insDataN)
		if err != nil {
			logger.Errorf("Error in generating SQL INSERT stmts for dependent/bridge table")
			return nil, err
		}
	}
	if len(insDataN) > 0 {
		res = &insDataN
	}

	return res, nil
}

func formReqdQueriesForEntity(insDataElem interface{}) ([]qInfo, error) {
	var err error

	logger.Debugf("mysql.Storage: formReqdQueriesForEntity")

	t := reflect.TypeOf(insDataElem).Elem()
	ename := t.Name()

	logger.Debugf("Entity Type: %v", t)

	qInfos := make([]qInfo, 0)
	// Loop around an entity type if it has any related entities that require
	// populating another tables in DB.
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Tag == "" {
			continue
		}
		relEntityNameKey := f.Tag.Get("entity.ukey")
		if relEntityNameKey == "" {
			// Only fields having relation indicated by entity.ukey are to be
			// processed.
			continue
		}
		stringSlice := strings.Split(relEntityNameKey, ".")
		logger.Debugf("entity.ukey split: %v", stringSlice)
		if len(stringSlice) != 2 {
			er := "entity.ukey tag is not in the required format"
			logger.Errorf("%s", er)
			err = errors.New(er)

			return nil, err
		}
		fKind := f.Type.Kind()
		if fKind != reflect.Slice && fKind != reflect.String {
			er := "entity.ukey field can only be slice of string or string"
			logger.Errorf("%s", er)
			err = errors.New(er)

			return nil, err
		}

		relEntityName := stringSlice[0]
		uKey := stringSlice[1]

		logger.Debugf("Entity: '%s' contains element with 'entity':'%s'",
			ename, relEntityName)

		tblsMap := map[string]bool{ename: true, relEntityName: true}
		logger.Debugf("Find relation between entities: %v", tblsMap)

		var relations *[]tblRelation = nil
		var bridgeTblIdx *[]int = nil

		relations, bridgeTblIdx, err = findTblRelations(tblsMap)
		if err != nil {
			logger.Errorf("Error in finding relations b/w tables")
			return nil, err
		}
		logger.Debugf("Relations: %v", *relations)

		q := qInfo{
			entityFieldIdx: i,
			tblToQueryIdx:  infraTblIdxMap[relEntityName],
			relations:      relations,
		}

		if bridgeTblIdx == nil {
			// Tables are directly related through Foreign key.
			logger.Debugf("Entities are directly related i.e. 1-1 relation exists")

			q.tblToUpdateIdx = infraTblIdxMap[ename]
		} else {
			// Tables are related through a bridge table containing Foreign keys of
			// both tables in tblsMap.
			logger.Debugf("Entities are NOT directly related i.e. M-M relation exists")
			logger.Debugf("Bridge Table Idx: %v", *bridgeTblIdx)

			brTblIdxV := *bridgeTblIdx
			q.tblToUpdateIdx = brTblIdxV[0]
		}

		tblToQueryIdx := infraTblIdxMap[relEntityName]
		qTblName := tables[tblToQueryIdx].Name
		uniqueKey := infraPropTblColMap[relEntityName][uKey]

		q.qStr = fmt.Sprintf("select %s.* from %s where %s.%s=?",
			qTblName, qTblName, qTblName, uniqueKey)

		logger.Debugf("Q: %v", q)
		qInfos = append(qInfos, q)
	}
	if len(qInfos) == 0 {
		logger.Debugf("No Query required for entity: '%s'", ename)
		return nil, nil
	} else {
		logger.Debugf("qInfos: %v", qInfos)
		return qInfos, nil
	}
}

func genSqlStmtsPerElem(tx *sql.Tx, insData *insert, qs []qInfo, insDataN *[]insert) error {

	logger.Debugf("mysql.Storage: genSqlStmtsPerElem")

	e := insData.Entities

	ename := reflect.TypeOf(e).Elem().Name()
	elems := reflect.ValueOf(e)

	logger.Debugf("Generating SQL Stmts for Entity type: '%s'", ename)

	// Loop around all elements of this entity.
	for i := 0; i < elems.Len(); i++ {
		logger.Debugf("Processing entity at Index: %d", i)
		ei := elems.Index(i).Interface()

		// Execute Queries inside transaction.
		for j := 0; j < len(qs); j++ {
			q := qs[j]

			qResults, qPkIds, err := executeQueriesForCreatingInsertStmts(ei, tx, &q)
			if err != nil {
				logger.Errorf("Error returned by executeQueriesForRelatedTbl()")
				return err
			}
			if qResults == nil {
				logger.Debugf("Slice of qResults: %v", qResults)
				continue
			}
			processQueryResultsPrepInsertStmts(insData, i, &q, qResults, qPkIds, insDataN)
		}
	}

	return nil
}

func processQueryResultsPrepInsertStmts(insData *insert, currIdx int, q *qInfo, qResults []interface{},
	qPkIds []interface{}, insDataN *[]insert) error {

	logger.Debugf("mysql.Storage: processQueryResultsPrepInsertStmts")

	for i, _ := range qResults {
		pkIds := qPkIds[i].([]int)
		logger.Debugf("Pk Ids: %v", pkIds)

		tbl := &tables[q.tblToUpdateIdx]
		vals := []interface{}{}
		valSubs := tbl.ValSubs

		switch q.tblToUpdateIdx {
		case computeNetworkTblIdx:
			logger.Debugf("Preparing Stmt(s) to be inserted in %s table", tbl.Name)

			var valSubsSet []string

			genId := insData.lastInsertId + int64(currIdx)
			for j, _ := range pkIds {
				logger.Debugf("Row %d: %d   %d", j, genId, pkIds[j])

				valSubsSet = append(valSubsSet, valSubs)

				vals = append(vals, genId, pkIds[j])
			}
			valSubs = strings.Join(valSubsSet, ",")
		default:
			er := fmt.Sprintf("Table Idx: %d not supported here", q.tblToUpdateIdx)
			logger.Errorf("%s", er)

			return errors.New(er)
		}
		if len(vals) == 0 {
			logger.Warnf("No rows received for %s table", tbl.Name)
			continue
		}

		logger.Debugf("SQL Stmt prepared for inserting rows in %s table", tbl.Name)
		addStmt := fmt.Sprintf(addStmt, tbl.Name, tbl.ColNames, valSubs)
		logger.Debugf("Statement: %s", addStmt)
		logger.Debugf("Values: %v", vals)

		ins := insert{
			Stmt: &addStmt,
			Vals: &vals,
		}

		*insDataN = append(*insDataN, ins)
	}
	return nil
}

func executeQueriesForCreatingInsertStmts(e interface{}, tx *sql.Tx, q *qInfo) ([]interface{}, []interface{}, error) {
	logger.Debugf("mysql.Storage: executeQueriesForCreatingInsertStmts")

	qResults := make([]interface{}, 0)
	qPkIds := make([]interface{}, 0)

	t := reflect.TypeOf(e)
	f := t.Field(q.entityFieldIdx)

	v := reflect.ValueOf(e)
	fv := v.Field(q.entityFieldIdx)

	switch f.Type.Kind() {
	case reflect.Slice:
		logger.Debugf("Field kind is slice")
		if fv.Len() == 0 {
			logger.Debugf("Skipping Query for Entity: '%s', because slice len for field: %d is 0",
				t.Name(), q.entityFieldIdx)
			return nil, nil, nil
		}
		// Loop around slice elements and execute queries.
		for i := 0; i < fv.Len(); i++ {
			result, pkIds, err := executeQueryForCreatingInsertStmt(q, fv.Index(i).Interface(), tx)
			if err != nil {
				logger.Errorf("Error returned by executeQueryForCreatingInsertStmt()")
				return nil, nil, err
			}
			if result == nil {
				er := "Error, SQL query returned 0 rows"
				logger.Errorf("%s", er)

				err = errors.New(er)
				return nil, nil, err
			}
			qResults = append(qResults, result)
			qPkIds = append(qPkIds, pkIds)
		}
	case reflect.String:
		logger.Debugf("Field type is string")
		result, pkIds, err := executeQueryForCreatingInsertStmt(q, fv.Interface(), tx)
		if err != nil {
			logger.Errorf("Error returned by executeQueryForCreatingInsertStmt()")
			return nil, nil, err
		}
		if result == nil {
			er := "Error, SQL query returned 0 rows"
			logger.Errorf("%s", er)

			err = errors.New(er)
			return nil, nil, err
		}
		qResults = append(qResults, result)
		qPkIds = append(qPkIds, pkIds)
	default:
		// Elements for query is of type string or slice of string only.
		er := "Elements for query is of type string or slice of string only"
		logger.Errorf("%s", er)

		return nil, nil, errors.New(er)
	}
	if len(qResults) == 0 {
		return nil, nil, nil
	} else {
		return qResults, qPkIds, nil
	}
}

func executeQueryForCreatingInsertStmt(q *qInfo, fvi interface{}, tx *sql.Tx) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: executeQueryForCreatingInsertStmt")

	ei := entity{q.tblToQueryIdx}

	vals := []interface{}{}
	vals = append(vals, fvi)

	logger.Debugf("Going to execute Query: %s with Vals: %v",
		q.qStr, vals)

	result, pkIds, err := executeQuery(q.qStr, ei, tx, vals...)
	if err != nil {
		logger.Errorf("Error returned by executeQuery()")
		return nil, nil, err
	}

	return result, pkIds, nil
}

func (tbl *table) genColNamesStr() string {
	logger.Debugf("mysql.Storage: genColNamesStr")

	colNames := "("
	colNames += strings.Join(tbl.Cols, ",")
	colNames += ")"

	logger.Debugf("mysql.Storage: Column Names String: %s", colNames)
	return colNames
}

func (tbl *table) genValSubsStr() string {
	logger.Debugf("mysql.Storage: genValSubs")

	var valStr []string

	valSubs := "("
	for range tbl.Cols {
		valStr = append(valStr, "?")
	}
	valSubs += strings.Join(valStr, ",")
	valSubs += ")"

	logger.Debugf("mysql.Storage: Column Substitute String: %s", valSubs)
	return valSubs
}

// Converts query's conditions into SQL string.
func clause(q *models.Query) string {
	logger.Debugf("mysql.Storage: clause")

	var op []string

	expr := ""

	createQueryCondsForEntity(q.Entity, &op)

	if len(q.Expr) != 0 {
		for _, v := range q.Expr {
			createQueryCondsForEntity(v, &op)
		}
	} else {
		logger.Debugf("Query clause(s) in Expr are empty")
	}

	expr = strings.Join(op, " ")
	logger.Debugf("Clause: %s", expr)

	return expr
}

func createQueryCondsForEntity(v interface{}, op *[]string) {
	logger.Debugf("mysql.Storage: createQueryCondsForEntity")

	t := reflect.TypeOf(v)

	switch t.Kind() {
	case reflect.String:
		val := reflect.ValueOf(v)
		*op = append(*op, val.Interface().(string))
	case reflect.Struct:
		var conds []string
		val := reflect.ValueOf(v)
		for i := 0; i < val.NumField(); i++ {
			cond := ""
			field := t.Field(i)
			switch field.Type.Name() {
			case "string":
				s := val.Field(i).Interface()
				if s != "" {
					cond = fmt.Sprintf("%s.%s = '%s'", tables[infraTblIdxMap[t.Name()]].Name,
						infraPropTblColMap[t.Name()][field.Name], s)
					logger.Debugf("Condition: %s", cond)
				}
			case "int":
				i := val.Field(i).Interface()
				if i != -1 {
					cond = fmt.Sprintf("%s.%s = %d", tables[infraTblIdxMap[t.Name()]].Name,
						infraPropTblColMap[t.Name()][field.Name], i)
					logger.Debugf("Condition: %s", cond)
				}
			}
			if cond != "" {
				conds = append(conds, cond)
			}
		}
		if len(conds) != 0 {
			concatConds := "(" + strings.Join(conds, " AND ") + ")"
			*op = append(*op, concatConds)
		}
	}
}

func getTblSetForQ(q *models.Query) (map[string]bool, error) {
	logger.Debugf("mysql.Storage: getTblSetForQ")

	// Query must be involving at the most two Infra entities.
	tbls := make(map[string]bool)

	entityName := reflect.TypeOf(q.Entity).Name()

	logger.Debugf("Query to find columns of Entity %s", entityName)

	// Add it in Map.
	tbls[entityName] = true

	// loop through all tables in expression and add them to tbls map.
	for _, v := range q.Expr {
		t := reflect.TypeOf(v)
		if t.Kind() == reflect.Struct {
			logger.Debugf("Query clause contains entity: %s", t.Name())

			// Add it in Map.
			tbls[t.Name()] = true
		}
	}

	logger.Debugf("Map of Tbls to be used in Query: %v", tbls)

	// Map will be of length 0, if clause is not there.
	return tbls, nil
}

// Checks if more tables are required, and if so, then update the map.
// Check for cases such as:
// 		- Tables A is related to B and B is related to C using Foreign keys.
// 		- Tables A and B are not related directly, but they are related
//		  through table C (having foreign keys referencing PKs in A and B).
func findTblRelations(m map[string]bool) (*[]tblRelation, *[]int, error) {
	logger.Debugf("mysql.Storage: findTblRelations")

	bridgeTblsReqd := false

	// Copy map of tables passed to this function.
	relationMap := make(map[string]bool)
	for i, v := range m {
		relationMap[i] = v
	}

	relations := make([]tblRelation, 0)

	// Check for relation like C references B and B references A exists b/w tables.
	// Check if these foreign keys point to the relation between the tables in m.
	for entityName, _ := range m {
		tblIdx, _ := infraTblIdxMap[entityName]
		tbl := tables[tblIdx]
		for fkIdx, k := range tbl.FK {

			logger.Debugf("Entity: '%s' found in map for FK: %v of Entity: '%s'",
				infraEntity[k.refTblIdx], k, entityName)

			if k.refTblIdx == tblIdx {
				logger.Debugf("Self-referential key: %v found in Tbl: %v, skipping it",
					k, tbl.Name)
				continue
			}

			_, found := relationMap[infraEntity[k.refTblIdx]]
			if !found {
				// Tbl is not present in map, so its relation is not relevant.
				logger.Debugf("'%s' Tbl is not present in map passed to this function, skipping it",
					infraEntity[k.refTblIdx])
				continue
			}

			referencedTbl := tables[k.refTblIdx]
			if referencedTbl.PK != k.refTblCol {
				// Key being reference is not the primary key of referenced table.
				logger.Debugf("Key: %s being referenced is not the primary key of referenced tbl: %s",
					k.refTblCol, referencedTbl.Name)
				continue
			}

			// Mark related tables as false in relationMap to indicate they are related.
			relationMap[entityName] = false
			relationMap[infraEntity[k.refTblIdx]] = false
			logger.Debugf("Marking tbls: '%s' & '%s' as related in relation map",
				entityName, infraEntity[k.refTblIdx])

			relations = append(relations, tblRelation{tblIdx, fkIdx})
		}
	}
	logger.Debugf("Resultant relation map: %v & Relations: %v", relationMap, relations)
	for _, v := range relationMap {
		if v == true {
			// Not all tables directly related, find bridge table(s) required to JOIN them"
			bridgeTblsReqd = true
			break
		}
	}
	if bridgeTblsReqd == false {
		logger.Debugf("Tables are related, no bridge table required to JOIN them")
		logger.Debugf("Relations: %v", relations)

		return &relations, nil, nil
	}

	bridgeTblIdx := make([]int, 0)

	logger.Debugf("Not all tables directly related, find bridge table(s) required to JOIN them")
	for tblIdx := computeNetworkTblIdx; tblIdx < lastTblIdx; tblIdx++ {
		tbl := tables[tblIdx]

		relationsTemp := make([]tblRelation, 0)
		keysToRem := make([]string, 0)

		for fkIdx, k := range tbl.FK {
			logger.Debugf("Checking FK: '%v' of bridge table: '%s'", k, tbl.Name)

			referencedTbl := tables[k.refTblIdx]
			if referencedTbl.PK != k.refTblCol {
				er := fmt.Sprintf("Bridge table is not referencing primary key of '%s' table",
					referencedTbl.Name)
				logger.Errorf("%s", er)
				return nil, nil, errors.New(er)
			}
			for key, _ := range relationMap {
				if infraEntity[k.refTblIdx] == key {
					logger.Debugf("Related Tbl: %s", referencedTbl.Name)

					relationsTemp = append(relationsTemp, tblRelation{tblIdx, fkIdx})
					keysToRem = append(keysToRem, key)
				}
			}
		}
		if len(relationsTemp) < 2 {
			logger.Debugf("Bridge table: %s doesn't relate any two tables in map: %v",
				tbl.Name, relationMap)
			continue
		}
		// len(relationsTemp) is equal to 2, it can't be greater than 2 as bridge table
		// don't have more than two columns.

		// Add relationsTemp to relations.
		relations = append(relations, relationsTemp...)

		// Remove tables from relationMap map.
		for _, key := range keysToRem {
			// Mark related tables as false in relationMap to indicate they are related.
			logger.Debugf("Marking tbl: '%s' as related in relation map", key)
			relationMap[key] = false
		}
		bridgeTblIdx = append(bridgeTblIdx, tblIdx)
	}
	logger.Debugf("Resultant copied map after checking bridge tables: %v", relationMap)
	for _, v := range relationMap {
		if v == true {
			er := "Unrelated tables are present"
			logger.Errorf("%s", er)

			return nil, nil, errors.New(er)
		}
	}
	logger.Debugf("Tables are related, Relations: %v, Bridge Tbl Indexes: %v",
		relations, bridgeTblIdx)

	return &relations, &bridgeTblIdx, nil
}

func convertRelationsToCondStr(r *[]tblRelation) string {
	logger.Debugf("mysql.Storage: convertRelationsToCondStr")
	str := ""

	conds := make([]string, 0)
	for _, v := range *r {
		var buffer bytes.Buffer

		fk := tables[v.tblIdx].FK[v.fkIdx]
		buffer.WriteString("(")
		buffer.WriteString(tables[v.tblIdx].Name)
		buffer.WriteString(".")
		buffer.WriteString(fk.col)
		buffer.WriteString(" = ")
		buffer.WriteString(tables[fk.refTblIdx].Name)
		buffer.WriteString(".")
		buffer.WriteString(fk.refTblCol)
		buffer.WriteString(")")

		logger.Debugf("Cond: %s", buffer.String())
		conds = append(conds, buffer.String())
	}
	if len(conds) != 0 {
		str += "("
		str += strings.Join(conds, " AND ")
		str += ")"
	}

	return str
}

func executeQuery(queryStr string, e entity, ctxt sqlQuery, vals ...interface{}) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: executeQuery")

	var rows *sql.Rows = nil
	var err error = nil

	rows, err = ctxt.Query(queryStr, vals...)
	if err != nil {
		logger.Errorf("%v", err)
		return nil, nil, err
	}
	defer rows.Close()

	ent, pkIds, err := e.Reader(rows)
	if err != nil {
		logger.Errorf("%v", err)
		return nil, nil, err
	}

	err = rows.Err()
	if err != nil {
		logger.Errorf("%v", err)
		return nil, nil, err
	}
	return ent, pkIds, nil
}

func dbExec(stmt string, db *sql.DB) error {
	logger.Debugf("mysql.Storage: dbExec")

	res, err := db.Exec(stmt)
	if err != nil {
		logger.Errorf("db.Exec returned error: %v", err)
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		logger.Errorf("Error in retrieving no. of affected rows: %v", err)
		return err
	}
	logger.Debugf("Rows affected: %d", rowsAffected)

	return nil
}

func (e *entity) Reader(rows *sql.Rows) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: Reader")

	var err error = nil
	var result interface{} = nil
	var pkIds interface{} = nil

	logger.Debugf("Table Idx passed to Reader: %d", e.tblIdx)

	switch e.tblIdx {
	case computeTblIdx:
		result, pkIds, err = scanComputeNodesRows(rows)
	case networkTblIdx:
		result, pkIds, err = scanNetworksRows(rows)
	case subnetTblIdx:
		result, pkIds, err = scanSubnetsRows(rows)
	case infraServiceTblIdx:
		result, pkIds, err = scanInfraServicesRows(rows)
	case securityGroupTblIdx:
		result, pkIds, err = scanSecurityGroupsRows(rows)
	case configTblIdx:
		result, pkIds, err = scanConfigsRows(rows)
	case flavorsTblIdx:
		result, pkIds, err = scanFlavorsRows(rows)
	case securityGrpRulesTblIdx:
		result, pkIds, err = scanSecurityGrpRulesRows(rows)
	default:
		er := "Table Index not found in switch-case"
		logger.Errorf("%s", er)

		err = errors.New(er)
		return nil, nil, err
	}

	return result, pkIds, err
}

func scanComputeNodesRows(rows *sql.Rows) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: scanComputeNodesRows")

	var err error = nil

	computes := make([]models.Compute, 0)
	pkIds := make([]int, 0)

	for rows.Next() {
		n := models.Compute{}
		pkId := 0

		err := rows.Scan(&pkId, &n.AvailZone, &n.Name, &n.Tier, &n.Vcpus, &n.RAM, &n.Disk)
		if err != nil {
			logger.Errorf("%v", err)
			return nil, nil, err
		}
		logger.Debugf("Row: PK Id: %d, Name: %s, Avail-Zone: %s, Tier-Level: %s, VCPUs: %d, RAM: %d, DISK: %d",
			pkId, n.Name, n.AvailZone, n.Tier, n.Vcpus, n.RAM, n.Disk)

		computes = append(computes, n)
		pkIds = append(pkIds, pkId)
	}
	if len(computes) == 0 {
		return nil, nil, err
	} else {
		return computes, pkIds, err
	}
}

func scanNetworksRows(rows *sql.Rows) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: scanNetworksRows")

	networks := make([]models.Network, 0)
	pkIds := make([]int, 0)

	for rows.Next() {
		n := models.Network{}
		pkId := 0

		err := rows.Scan(&pkId, &n.Identifier, &n.Category)
		if err != nil {
			logger.Errorf("%v", err)
			return nil, nil, err
		}
		logger.Debugf("Row: PK Id: %d, OS Id: %s, Category: %s",
			pkId, n.Identifier, n.Category)

		networks = append(networks, n)
		pkIds = append(pkIds, pkId)
	}
	if len(networks) == 0 {
		return nil, nil, nil
	} else {
		return networks, pkIds, nil
	}
}

func scanSubnetsRows(rows *sql.Rows) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: scanSubnetsRows")

	subnets := make([]models.Subnet, 0)
	pkIds := make([]int, 0)

	for rows.Next() {
		n := models.Subnet{}
		pkId := 0

		err := rows.Scan(&pkId, &n.Identifier, &n.Category)
		if err != nil {
			logger.Errorf("%v", err)
			return nil, nil, err
		}
		logger.Debugf("Row: PK Id: %d, OS Id: %s, Category: %s",
			pkId, n.Identifier, n.Category)

		subnets = append(subnets, n)
		pkIds = append(pkIds, pkId)
	}
	if len(subnets) == 0 {
		return nil, nil, nil
	} else {
		return subnets, pkIds, nil
	}
}

func scanInfraServicesRows(rows *sql.Rows) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: scanInfraServicesRows")

	infraServices := make([]models.InfraService, 0)
	pkIds := make([]int, 0)

	for rows.Next() {
		n := models.InfraService{}
		pkId := 0

		err := rows.Scan(&pkId, &n.ServiceType, &n.Value)
		if err != nil {
			logger.Errorf("%v", err)
			return nil, nil, err
		}
		logger.Debugf("Row: PK Id: %d, OS Id: %s, Value: %s",
			pkId, n.ServiceType, n.Value)

		infraServices = append(infraServices, n)
		pkIds = append(pkIds, pkId)
	}
	if len(infraServices) == 0 {
		return nil, nil, nil
	} else {
		return infraServices, pkIds, nil
	}
}

func scanSecurityGroupsRows(rows *sql.Rows) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: scanSecurityGroupsRows")

	secGrps := make([]models.SecurityGroup, 0)
	pkIds := make([]int, 0)

	for rows.Next() {
		n := models.SecurityGroup{}
		pkId := 0

		err := rows.Scan(&pkId, &n.Identifier, &n.Category)
		if err != nil {
			logger.Errorf("%v", err)
			return nil, nil, err
		}
		logger.Debugf("Row: PK Id: %d, OS Id: %s, Category: %s",
			pkId, n.Identifier, n.Category)

		secGrps = append(secGrps, n)
		pkIds = append(pkIds, pkId)
	}
	if len(secGrps) == 0 {
		return nil, nil, nil
	} else {
		return secGrps, pkIds, nil
	}
}

func scanConfigsRows(rows *sql.Rows) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: scanConfigsRows")

	configs := make([]models.Config, 0)

	for rows.Next() {
		n := models.Config{}

		err := rows.Scan(&n.ConfKey, &n.Value)
		if err != nil {
			logger.Errorf("%v", err)
			return nil, nil, err
		}
		logger.Debugf("Row: Conf Key: %s, Value: %s", n.ConfKey, n.Value)

		configs = append(configs, n)
	}
	if len(configs) == 0 {
		return nil, nil, nil
	} else {
		return configs, nil, nil
	}
}

func scanFlavorsRows(rows *sql.Rows) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: scanFlavorsRows")

	var err error = nil

	flavors := make([]models.Flavor, 0)

	for rows.Next() {
		n := models.Flavor{}

		err := rows.Scan(&n.Name, &n.Vcpus, &n.RAM, &n.Disk)
		if err != nil {
			logger.Errorf("%v", err)
			return nil, nil, err
		}
		logger.Debugf("Row: Name: %s, Vcpus: %d, RAM: %d, Disk: %d",
			n.Name, n.Vcpus, n.RAM, n.Disk)

		flavors = append(flavors, n)
	}
	if len(flavors) == 0 {
		return nil, nil, err
	} else {
		return flavors, nil, err
	}
}

func scanSecurityGrpRulesRows(rows *sql.Rows) (interface{}, interface{}, error) {
	logger.Debugf("mysql.Storage: scanSecurityGrpRulesRows")

	var err error = nil

	securityGrpRules := make([]models.SecurityGrpRule, 0)

	for rows.Next() {
		n := models.SecurityGrpRule{}

		var port sql.NullInt64

		err := rows.Scan(&n.Name, &n.Protocol, &port)
		if err != nil {
			logger.Errorf("%v", err)
			return nil, nil, err
		}

		if port.Valid {
			n.Port = int(port.Int64)
		} else {
			n.Port = 0
		}

		logger.Debugf("Row: Name: %s, Protocol: %s, Port: %d",
			n.Name, n.Protocol, n.Port)

		securityGrpRules = append(securityGrpRules, n)
	}
	if len(securityGrpRules) == 0 {
		return nil, nil, err
	} else {
		return securityGrpRules, nil, err
	}
}
