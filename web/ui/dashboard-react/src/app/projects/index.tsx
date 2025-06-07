import {useState} from 'react';
import {createFileRoute, redirect, useNavigate} from '@tanstack/react-router';

import {Button} from '@/components/ui/button';
import {ConvoyLoader} from '@/components/convoy-loader';
import {CreateOrganisation} from '@/components/create-organisation';

import {useLicenseStore, useOrganisationStore, useProjectStore,} from '@/store';
import * as authService from '@/services/auth.service';
import {ensureCanAccessPrivatePages} from '@/lib/auth';
import * as projectsService from '@/services/projects.service';
import * as orgsService from '@/services/organisations.service';

import plusCircularIcon from '../../../assets/svg/add-circlar-icon.svg';
import projectsEmptyImg from '../../../assets/svg/events-empty-state-image.svg';

export const Route = createFileRoute('/projects/')({
  beforeLoad({context}) {
    ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
  },
  async loader() {
    let pgOrgs;
    try {
      pgOrgs = await orgsService.getOrganisations();
    } catch (error) {
      console.error('Error loading organisations:', error);
      return {
        canCreateProject: false,
        canCreateOrg: useLicenseStore.getState().licenses.includes('CREATE_ORG'),
      };
    }
    const {org, setPaginatedOrgs, setOrg} = useOrganisationStore.getState();
    // Only update the store if we don't have an org or if the current org is not in the list
    if (!org || !pgOrgs.content.some(o => o.uid === org.uid)) {
      setPaginatedOrgs(pgOrgs);
      setOrg(pgOrgs.content.at(0) || null);
    }

    // Re-read the org from the store after setting
    const currentOrg = useOrganisationStore.getState().org;
    console.log('Org in store before getProjects:', currentOrg);

    // If we still don't have an org after trying to set it, we need to show the create org UI
    if (!currentOrg) {
      console.log('No org found, returning early from loader.');
      return {
        canCreateProject: false,
        canCreateOrg: useLicenseStore.getState().licenses.includes('CREATE_ORG'),
      };
    }

    // Only proceed if org exists
    try {
      const projects = await projectsService.getProjects();
      const {setProjects, setProject} = useProjectStore.getState();
      setProjects(projects);

      if (!useProjectStore.getState().project) {
        setProject(projects.at(0) || null);
      }

      if (projects.length > 0) {
        throw redirect({
          to: '/projects/$projectId/events',
          params: {projectId: projects[0].uid},
        });
      }

      const userPerms = await authService.getUserPermissions();

      return {
        canCreateProject: userPerms.includes('Project Settings|MANAGE'),
        canCreateOrg: useLicenseStore.getState().licenses.includes('CREATE_ORG'),
      };
    } catch (error) {
      console.error('Error loading projects:', error);
      return {
        canCreateProject: false,
        canCreateOrg: useLicenseStore.getState().licenses.includes('CREATE_ORG'),
      };
    }
  },
  component: ProjectIndexPage,
});

function ProjectIndexPage() {
  const navigate = useNavigate();
  const {project} = useProjectStore();
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const {canCreateOrg, canCreateProject} = Route.useLoaderData();
  const {org, setOrg, setPaginatedOrgs} = useOrganisationStore();
  const [isLoadingOrganisations, setIsLoadingOrganisations] = useState(false);

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
    return <ConvoyLoader isTransparent={true} isVisible={true}/>;

  if (!org)
    return (
      <CreateOrganisation
        onOrgCreated={reloadOrganisations}
        isDialogOpen={isDialogOpen}
        setIsDialogOpen={setIsDialogOpen}
      >
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
      </CreateOrganisation>
    );

  if (!project) {
    return (
      <div className="flex flex-col items-center p-6">
        <img
          src={projectsEmptyImg}
          alt={'no projects created for ' + org.name}
          className="h-40 mb-12"
        />
        <h2 className="mb-4 font-bold text-base text-neutral-12">
          Create a project to get started
        </h2>
        <p className="text-sm text-neutral-10">
          Your incoming and outgoing projects appear here.
        </p>
        <Button
          disabled={!canCreateProject}
          onClick={() => {
            navigate({to: '/projects/new'});
          }}
          variant="ghost"
          className="flex justify-center items-center hover:bg-new.primary-400 hover:text-white-100 bg-new.primary-400 px-3 py-4 mt-9"
        >
          <img
            className="w-[20px] h-[20px]"
            src={plusCircularIcon}
            alt="create project"
          />
          <p className="text-white-100 text-xs">Create a Project</p>
        </Button>
      </div>
    );
  }
}

// FIXME: Semantic HTML says anchors shoudld be anchors and buttons, buttons.
// Style an anchor tag like a button if you want it to look like one.
// See https://stackoverflow.com/q/64443645
