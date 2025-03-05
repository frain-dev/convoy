import { useState, useEffect } from 'react';
import { createFileRoute } from '@tanstack/react-router';

import { ConvoyLoader } from '@/components/convoy-loader';
import { CreateOrganisation } from '@/components/create-organisation';

import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as projectsService from '@/services/projects.service';
import * as orgsService from '@/services/organisations.service';

import type { Project } from '@/models/project.model';
import type { Organisation } from '@/models/organisation.model';

export const Route = createFileRoute('/projects/_layout/')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	component: RouteComponent,
});

function RouteComponent() {
	const [isLoadingOrganisations, setIsLoadingOrganisations] = useState(false);
	const [organisations, setOrganisations] = useState<Organisation[]>([]);

	// TODO use a hook for organisations and projects
	const [currentProject /* setCurrentProject */] = useState<Project | null>(
		projectsService.getCachedProject(),
	);

	useEffect(() => {
		getOrganisations();

		return () => {
			// clear all requests on unmount component
		};
	}, []);

	function getOrganisations() {
		setIsLoadingOrganisations(true);
		orgsService
			.getOrganisations({ refresh: true })
			.then(({ content }) => setOrganisations(content))
			// TODO use toast component to show UI error on all catch(error) where necessary
			.catch(console.error)
			.finally(() => {
				setIsLoadingOrganisations(false);
			});
	}

	if (isLoadingOrganisations) return <ConvoyLoader isTransparent={true} />;

	if (organisations.length == 0)
		return (
			<CreateOrganisation
				onOrgCreated={() => {
					console.log('org created');
					getOrganisations();
				}}
			/>
		);

	if (!currentProject) {
		return (
			<p className="font-semibold text-xl">
				TODO: create create project component
			</p>
		);
	}

	return <p className="text-3xl-font-bold">TODO: create project default UI</p>;
}
