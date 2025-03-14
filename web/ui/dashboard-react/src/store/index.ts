import { create } from 'zustand';

import type { Project } from '@/models/project.model';
import type { PaginatedResult } from '@/models/global.model';
import type { LicenseKey } from '@/services/licenses.service';
import type { Organisation } from '@/models/organisation.model';

type LicenseStore = {
	licenses: Array<LicenseKey>;
	setLicenses: (keys: Array<LicenseKey>) => void;
};

export const useLicenseStore = create<LicenseStore>()(set => ({
	licenses: [],
	setLicenses: keys => set({ licenses: keys }),
}));

type OrganisationStore = {
	/** The current organisation in use */
	org: Organisation | null;
	/** Set the current organisation in use */
	setOrg: (org: Organisation | null) => void;
	paginatedOrgs: PaginatedResult<Organisation>;
	setPaginatedOrgs: (pgOrgs: PaginatedResult<Organisation>) => void;
};

export const useOrganisationStore = create<OrganisationStore>()(set => ({
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
}));

type ProjectStore = {
	/** The current project in use */
	project: Project | null;
	projects: Array<Project>;
	/** Set the current project in use */
	setProject: (p: Project | null) => void;
	setProjects: (projects: Array<Project>) => void;
};

export const useProjectStore = create<ProjectStore>()(set => ({
	project: null,
	projects: [],
	setProject: project => set({ project }),
	setProjects: projects => set({ projects }),
}));
