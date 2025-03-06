import { useState } from 'react';
import { createFileRoute } from '@tanstack/react-router';

import { Button } from '@/components/ui/button';
import { ConvoyLoader } from '@/components/convoy-loader';
import { CreateOrganisation } from '@/components/create-organisation';

import { useOrganisationContext } from '@/contexts/organisation';

import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as projectsService from '@/services/projects.service';
import * as licensesService from '@/services/licenses.service';
import * as orgsService from '@/services/organisations.service';

import plusCircularIcon from '../../../assets/svg/add-circlar-icon.svg';

import type { Project } from '@/models/project.model';

export const Route = createFileRoute('/projects/')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	component: RouteComponent,
});

function RouteComponent() {
	const [isDialogOpen, setIsDialogOpen] = useState(false);
	const { setOrganisations, organisations } = useOrganisationContext();
	const [canCreateOrg] = useState(licensesService.hasLicense('CREATE_ORG'));
	const [isLoadingOrganisations, setIsLoadingOrganisations] = useState(false);

	// TODO use a state management lib for projects
	const [currentProject] = useState<Project | null>(
		projectsService.getCachedProject(),
	);

	function getOrganisations() {
		setIsLoadingOrganisations(true);
		orgsService
			.getOrganisations({ refresh: true })
			.then(({ content }) => {
				setOrganisations(content);
			})
			// TODO use toast component to show UI error on all catch(error) where necessary
			.catch(console.error)
			.finally(() => {
				setIsLoadingOrganisations(false);
			});
	}

	if (isLoadingOrganisations)
		return <ConvoyLoader isTransparent={true} isVisible={true} />;

	if (organisations.length == 0)
		return (
			<CreateOrganisation
				onOrgCreated={() => {
					getOrganisations();
				}}
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
