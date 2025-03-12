import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

import {
	CONVOY_LICENSES_KEY,
	CONVOY_ORG_KEY,
	CONVOY_CURRENT_PROJECT,
} from '@/lib/constants';

import type { Project } from '@/models/project.model';
import type { PaginatedResult } from '@/models/global.model';
import type { LicenseKey } from '@/services/licenses.service';
import type { Organisation } from '@/models/organisation.model';

// window.sessionStorage is used so that if the page is refreshed, app will use new data from server

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
			storage: createJSONStorage(() => sessionStorage),
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
			storage: createJSONStorage(() => sessionStorage),
		},
	),
);

type ProjectStore = {
	/** The current project in use */
	project: Project | null;
	projects: Array<Project>;
	/** Set the current project in use */
	setProject: (p: Project | null) => void;
	setProjects: (projects: Array<Project>) => void;
};

export const useProjectStore = create<ProjectStore>()(
	persist(
		set => ({
			project: null,
			projects: [],
			setProject: project => set({ project }),
			setProjects: projects => set({ projects }),
		}),
		{
			name: CONVOY_CURRENT_PROJECT,
			storage: createJSONStorage(() => sessionStorage),
		},
	),
);
