import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

import { CONVOY_LICENSES_KEY, CONVOY_ORG_KEY } from '@/lib/constants';

import type { PaginatedResult } from '@/models/global.model';
import type { LicenseKey } from '@/services/licenses.service';
import type { Organisation } from '@/models/organisation.model';

type LicenseStore = {
	licenses: Array<LicenseKey>;
	setLicenses: (keys: Array<LicenseKey>) => void;
};

export const useLicenseStore = create<LicenseStore>()(
	persist(
		set => ({
			licenses: [],
			setLicenses: keys => set({ licenses: keys }),
		}),
		{
			name: CONVOY_LICENSES_KEY,
			storage: createJSONStorage(() => localStorage), // just to be explicit
		},
	),
);

type OrganisationStore = {
	/** The current organisation in use */
	org: Organisation | null;
	/** Set the current organisation in use */
	setOrg: (org: Organisation | null) => void;
	paginatedOrgs: PaginatedResult<Organisation>;
	setPaginatedOrgs: (pgOrgs: PaginatedResult<Organisation>) => void;
};

export const useOrganisationStore = create<OrganisationStore>()(
	persist(
		set => ({
			org: null,
			setOrg: org => set({ org }),
			paginatedOrgs: {
				content: [],
				pagination: {
					per_page: 0,
					has_next_page: false,
					has_prev_page: false,
					prev_page_cursor: '',
					next_page_cursor: '',
				},
			},
			setPaginatedOrgs: pgOrgs => set({ paginatedOrgs: pgOrgs }),
		}),
		{
			name: CONVOY_ORG_KEY,
		},
	),
);
