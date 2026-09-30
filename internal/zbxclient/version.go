package zbxclient

import (
	"context"
	"fmt"
)

// VersionAtLeast reports whether Zabbix API version v (e.g. "7.4.14") is at
// least major.minor. Unparseable versions compare as false.
func VersionAtLeast(v string, major, minor int) bool {
	var ma, mi int
	if _, err := fmt.Sscanf(v, "%d.%d", &ma, &mi); err != nil {
		return false
	}
	return ma > major || (ma == major && mi >= minor)
}

// APIAtLeast checks the server version; write paths use it and must stop
// rather than guess parameter names when the version is unknown.
func (c *Client) APIAtLeast(ctx context.Context, major, minor int) (bool, error) {
	v, err := c.APIVersion(ctx)
	if err != nil {
		return false, fmt.Errorf("apiinfo.version: %w", err)
	}
	var ma, mi int
	if _, err := fmt.Sscanf(v, "%d.%d", &ma, &mi); err != nil {
		return false, fmt.Errorf("unrecognised Zabbix API version %q; refusing to guess API parameters", v)
	}
	return VersionAtLeast(v, major, minor), nil
}
