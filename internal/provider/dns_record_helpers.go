package provider

import (
	"fmt"
	"strings"
	"sync"

	"github.com/bearcode33/terraform-provider-godaddy/internal/godaddy"
)

// recordSetLocks serializes read-modify-write on one domain/type/name set.
// The v1 API replaces a whole set at once, and Terraform runs resources in
// parallel, so two resources on the same set could each write an old copy
// and drop the other's record.
var recordSetLocks sync.Map

func lockRecordSet(domain, recordType, name string) func() {
	key := strings.ToLower(domain) + "/" + strings.ToUpper(recordType) + "/" + strings.ToLower(name)
	value, _ := recordSetLocks.LoadOrStore(key, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// singleValueRecordTypes can hold only one record per name. Every other type
// can hold several records at the same name (several TXT or MX records, for
// example).
var singleValueRecordTypes = map[string]bool{
	godaddy.RecordTypeCNAME: true,
	godaddy.RecordTypeSOA:   true,
	godaddy.RecordTypeDNAME: true,
}

func isSingleValueType(recordType string) bool {
	return singleValueRecordTypes[strings.ToUpper(recordType)]
}

// hostnameRecordTypes hold a hostname in data. DNS hostnames ignore case,
// and the API may return them with or without a trailing dot.
var hostnameRecordTypes = map[string]bool{
	godaddy.RecordTypeCNAME: true,
	godaddy.RecordTypeMX:    true,
	godaddy.RecordTypeNS:    true,
	godaddy.RecordTypePTR:   true,
	godaddy.RecordTypeSRV:   true,
	godaddy.RecordTypeDNAME: true,
}

// sameRecordData compares record data the way DNS does. Without this, a
// hostname the API returns as "Mail.Example.com." would not match the config's
// "mail.example.com": refresh would drop the record from state and the next
// apply would add a duplicate.
func sameRecordData(recordType, a, b string) bool {
	if hostnameRecordTypes[strings.ToUpper(recordType)] {
		return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
	}
	return a == b
}

type createAction int

const (
	// createAdd appends the record and leaves existing records alone.
	createAdd createAction = iota
	// createAdopt rewrites the existing record that has the same data, so
	// its TTL and other fields match the config. Other records stay.
	createAdopt
	// createReplace replaces a single-value record with the new one.
	createReplace
)

// planCreate decides how Create writes desired when existing records are
// already at the same type and name. It never picks an action that deletes a
// record of a multi-value type.
func planCreate(existing []godaddy.DNSRecord, desired godaddy.DNSRecord, allowOverwrite bool) (createAction, error) {
	if len(existing) == 0 {
		return createAdd, nil
	}

	for _, record := range existing {
		if sameRecordData(desired.Type, record.Data, desired.Data) {
			if allowOverwrite {
				return createAdopt, nil
			}
			return 0, fmt.Errorf("a %s record %q with data %q already exists. "+
				"Import it with terraform import, or set allow_overwrite = true to manage the existing record",
				desired.Type, desired.Name, desired.Data)
		}
	}

	if !isSingleValueType(desired.Type) {
		return createAdd, nil
	}

	if allowOverwrite {
		return createReplace, nil
	}

	var existingData []string
	for _, record := range existing {
		existingData = append(existingData, record.Data)
	}
	return 0, fmt.Errorf("a %s record %q already exists with data [%s], and a name can hold only one %s record. "+
		"Import it with terraform import, or set allow_overwrite = true to replace it",
		desired.Type, desired.Name, strings.Join(existingData, ", "), desired.Type)
}

// findStoredRecord picks the live record that matches the record in state.
// It matches on data. A single-value type has at most one record at the name,
// so a record with other data is the same record changed outside Terraform
// and is returned so the plan shows the change. For other types a missing
// match returns false: the other records at that name belong to someone else.
func findStoredRecord(records []godaddy.DNSRecord, recordType, storedData string) (*godaddy.DNSRecord, bool) {
	for i := range records {
		if sameRecordData(recordType, records[i].Data, storedData) {
			return &records[i], true
		}
	}
	if isSingleValueType(recordType) && len(records) == 1 {
		return &records[0], true
	}
	return nil, false
}
