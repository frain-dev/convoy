import { createContext, useContext, useEffect, useState } from 'react';

import { CONVOY_ORG_KEY } from '@/lib/constants';

import type { Organisation } from '@/models/organisation.model';
import type { ReactNode, SetStateAction, Dispatch } from 'react';

function getCachedOrganisation(): Organisation | null {
	let org = localStorage.getItem(CONVOY_ORG_KEY);
	return org ? JSON.parse(org) : null;
}

export type OrganisationContext = {
	organisations: Organisation[];
	setOrganisations: Dispatch<SetStateAction<Organisation[]>>;
	currentOrganisation: Organisation | null;
	setCurrentOrganisation: Dispatch<SetStateAction<Organisation | null>>;
};

const OrganisationContext = createContext<OrganisationContext>({
	currentOrganisation: getCachedOrganisation(),
	organisations: [],
	setCurrentOrganisation: () => null,
	setOrganisations: () => [],
});
OrganisationContext.displayName = 'OrganisationContext';

export const useOrganisationContext = () => useContext(OrganisationContext);

export function OrganisationProvider({ children }: { children: ReactNode }) {
	const [organisations, setOrganisations] = useState<Organisation[]>([]);
	const [currentOrganisation, setCurrentOrganisation] = useState(
		getCachedOrganisation(),
	);

	useEffect(() => {
		if (!organisations.length) {
			setCurrentOrganisation(null);
			return localStorage.removeItem(CONVOY_ORG_KEY);
		}

		setCurrentOrganisation(organisations[0]);
		localStorage.setItem(CONVOY_ORG_KEY, JSON.stringify(organisations[0]));
	}, [organisations]);

	return (
		<OrganisationContext.Provider
			value={{
				currentOrganisation,
				organisations,
				setCurrentOrganisation,
				setOrganisations,
			}}
		>
			{children}
		</OrganisationContext.Provider>
	);
}

// TODO use Zustand for state management
