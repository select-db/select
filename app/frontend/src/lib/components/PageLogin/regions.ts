// Prod region catalog. Only `available` regions map to a live server domain;
// the rest render as "Coming soon". Adding a region later: flip `available`
// and fill `domain` with its api host. Selecting a region just calls
// SetCurrentServer(domain) (idempotent, creates + migrates the local DB if
// missing), so no extra wiring is needed when a new region goes live.
export type FlagCode = 'us' | 'eu';

export interface Region {
	id: string;
	/** Short region name, e.g. "US East". */
	label: string;
	/** Physical location shown under the label, e.g. "Virginia". */
	location: string;
	flag: FlagCode;
	/** Live api host, or null while the region is not yet launched. */
	domain: string | null;
	available: boolean;
}

export const REGIONS: Region[] = [
	{
		id: 'us-east',
		label: 'US East',
		location: 'Virginia',
		flag: 'us',
		domain: 'api.us-east.select-db.com',
		available: true
	},
	{
		// Planned host when launched: api.us-west.select-db.com
		id: 'us-west',
		label: 'US West',
		location: 'Oregon',
		flag: 'us',
		domain: null,
		available: false
	},
	{
		// Planned host when launched: api.eu.select-db.com
		id: 'eu',
		label: 'Europe',
		location: 'Gravelines',
		flag: 'eu',
		domain: null,
		available: false
	}
];
