import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';

import { CONVOY_LICENSES_KEY } from '@/lib/constants';

import type { LicenseKey } from '@/services/licenses.service';

type LicenseStore = {
	licenses: Array<LicenseKey>;
	setLicenses: (keys: Array<LicenseKey>) => void;
};

export const useLicenseStore = create<LicenseStore>()(
	persist(
		set => ({
			licenses: [],
			setLicenses: (keys: Array<LicenseKey>) => set({ licenses: keys }),
		}),
		{
			name: CONVOY_LICENSES_KEY,
			storage: createJSONStorage(() => localStorage), // just to be explicit
		},
	),
);
