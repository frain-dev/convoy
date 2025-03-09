import { useState } from 'react';
import { createFileRoute } from '@tanstack/react-router';

import { Button } from '@/components/ui/button';
import { ConvoyLoader } from '@/components/convoy-loader';
import { CreateOrganisation } from '@/components/create-organisation';

import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as projectsService from '@/services/projects.service';
import { useLicenseStore, useOrganisationStore } from '@/store';
import * as orgsService from '@/services/organisations.service';

import plusCircularIcon from '../../../assets/svg/add-circlar-icon.svg';

import type { Project } from '@/models/project.model';

export const Route = createFileRoute('/projects/')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	async loader() {
		const { org, paginatedOrgs } = useOrganisationStore.getState();

		if (!org || !paginatedOrgs.content.length) {
			const pgOrgs = await orgsService.getOrganisations();

			useOrganisationStore.setState({
				paginatedOrgs: pgOrgs,
				org: pgOrgs.content.at(0) || null,
			});
		}
	},
	component: RouteComponent,
});

function RouteComponent() {
	const { licenses } = useLicenseStore();
	const [isDialogOpen, setIsDialogOpen] = useState(false);
	const { org, setOrg, setPaginatedOrgs } = useOrganisationStore();
	const [canCreateOrg] = useState(licenses.includes('CREATE_ORG'));
	const [isLoadingOrganisations, setIsLoadingOrganisations] = useState(false);
	// TODO use a state management lib for projects like zustand
	const [currentProject] = useState<Project | null>(
		projectsService.getCachedProject(),
	);

	async function reloadOrganisations() {
		setIsLoadingOrganisations(true);
		orgsService
			.getOrganisations()
			.then(pgOrgs => {
				setPaginatedOrgs(pgOrgs);
				setOrg(pgOrgs.content.at(0) || null);
			})
			// TODO use toast component to show UI error on all catch(error) where necessary
			.catch(console.error)
			.finally(() => {
				setIsLoadingOrganisations(false);
			});
	}

	if (isLoadingOrganisations)
		return <ConvoyLoader isTransparent={true} isVisible={true} />;

	if (!org)
		return (
			<CreateOrganisation
				onOrgCreated={reloadOrganisations}
				isDialogOpen={isDialogOpen}
				setIsDialogOpen={setIsDialogOpen}
				children={
					<Button
						disabled={!canCreateOrg}
						onClick={() => {
							setIsDialogOpen(isOpen => !isOpen);
						}}
						variant="ghost"
						className="flex justify-center items-center hover:bg-new.primary-400 hover:text-white-100 bg-new.primary-400 mt-10"
					>
						<img
							className="w-[20px] h-[20px]"
							src={plusCircularIcon}
							alt="create organisation"
						/>
						<p className="text-white-100 text-xs">Create Organisation</p>
					</Button>
				}
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
