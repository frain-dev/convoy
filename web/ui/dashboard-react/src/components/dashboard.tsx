import { z } from 'zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useNavigate, Link } from '@tanstack/react-router';

import {
	SidebarIcon,
	ChevronDown,
	CirclePlusIcon,
	SettingsIcon,
	BookOpen,
	Check,
	ChevronsUpDown,
	HelpCircle,
} from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { CreateOrganisationDialog } from './create-organisation';
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
	useSidebar,
	Sidebar,
	SidebarGroup,
	SidebarContent,
	SidebarFooter,
	SidebarGroupContent,
	SidebarProvider,
	SidebarInset,
} from '@/components/ui/sidebar';
import {
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from '@/components/ui/tooltip';
import {
	Popover,
	PopoverTrigger,
	PopoverContent,
} from '@/components/ui/popover';
import {
	Command,
	CommandInput,
	CommandList,
	CommandEmpty,
	CommandGroup,
	CommandItem,
} from '@/components/ui/command';
import { FormField, FormItem, FormControl, Form } from '@/components/ui/form';

import { cn } from '@/lib/utils';
import { Route } from '@/app/__root';
import * as transform from '@/lib/pipes';
import * as authService from '@/services/auth.service';
import * as projectsService from '@/services/projects.service';
import * as orgsService from '@/services/organisations.service';
import {
	useLicenseStore,
	useOrganisationStore,
	useProjectStore,
} from '@/store';

import convoyLogo from '../../assets/svg/logo.svg';
import userProfileIcon from '../../assets/svg/user-icon.svg';

import type { ComponentProps, ReactNode } from 'react';
import type { Organisation } from '@/models/organisation.model';

function HeaderRightProfile() {
	const { auth } = Route.useRouteContext();
	const currentUser = auth?.getCurrentUser() || null;

	if (!currentUser)
		return (
			<li>
				{/* TODO code this out */}
				<span>skeleton</span>
			</li>
		);

	return (
		<li>
			<DropdownMenu>
				<DropdownMenuTrigger asChild>
					<Button
						variant="ghost"
						className="bg-neutral-3 shadow-none focus-visible:ring-0"
					>
						<img src={userProfileIcon} alt="user profile dropdown" />
					</Button>
				</DropdownMenuTrigger>
				<DropdownMenuContent className="w-64" align="end">
					<DropdownMenuGroup className="pt-2 pb-2 px-3">
						<div className="flex flex-col">
							<p className="capitalize text-sm font-medium truncate p-0 hover:bg-transparent hover:cursor-default">
								{currentUser?.first_name || ''} {currentUser?.last_name || ''}
							</p>
							<p className="text-neutral-11 text-xs p-0 mt-1 hover:bg-transparent hover:cursor-default">
								{currentUser?.email}
							</p>
						</div>
					</DropdownMenuGroup>
					<DropdownMenuSeparator />
					<DropdownMenuItem className="hover:cursor-pointer hover:bg-neutral-3 pl-3">
						<Link
							to="/user-settings"
							className="block w-full text-xs text-neutral-11"
						>
							My account
						</Link>
					</DropdownMenuItem>
					<DropdownMenuSeparator />
					<DropdownMenuItem
						className="hover:cursor-pointer hover:bg-neutral-3 pl-3"
						onClick={authService.logUserOut}
					>
						<span className="text-destructive hover:text-destructive text-xs">
							Logout
						</span>
					</DropdownMenuItem>
				</DropdownMenuContent>
			</DropdownMenu>
		</li>
	);
}

function HeaderRightOrganisation() {
	const { licenses } = useLicenseStore();
	const navigate = useNavigate({ from: Route.fullPath });
	const { org, paginatedOrgs, setOrg, setPaginatedOrgs } =
		useOrganisationStore();
	const { setProjects, setProject } = useProjectStore();
	const [isDialogOpen, setIsDialogOpen] = useState(false);
	const [canCreateOrg] = useState(licenses.includes('CREATE_ORG'));

	async function reloadProjects(org: Organisation | null) {
		setOrg(org);
		const projects = await projectsService.getProjects();
		setProjects(projects);
		setProject(projects.at(0) || null);
	}

	async function reloadOrganisations() {
		orgsService
			.getOrganisations()
			.then(pgOrgs => {
				setPaginatedOrgs(pgOrgs);
				setOrg(pgOrgs.content.at(0) || null);
			})
			// TODO use toast component to show UI error on all catch(error) where necessary
			.catch(console.error);
	}

	if (!org) return null;

	return (
		<li className="mr-3">
			<DropdownMenu>
				<DropdownMenuTrigger asChild>
					<Button
						variant="ghost"
						className="bg-neutral-3 shadow-none px-5 flex justify-start items-center focus-visible:ring-0"
					>
						<Avatar className="rounded-[100%] w-6 h-6">
							<AvatarFallback className="bg-new.primary-600 text-white-100 text-[10px]">
								{transform.getInitials(org.name.split(' '))}
							</AvatarFallback>
						</Avatar>
						<span className="text-xs block px-1">{org?.name}</span>
						<ChevronDown />
					</Button>
				</DropdownMenuTrigger>

				<DropdownMenuContent align="end" className="w-64">
					<DropdownMenuGroup className="py-1">
						<DropdownMenuItem className="focus:bg-transparent text-xs font-semibold focus:text-neutral-11 text-neutral-11 truncate py-1 hover:bg-transparent hover:cursor-default">
							Your organisations ({paginatedOrgs.content.length})
						</DropdownMenuItem>
						{/* TODO there is a 'padlockicon Business' overlay here. Check and create */}
					</DropdownMenuGroup>

					<DropdownMenuSeparator />

					<DropdownMenuGroup>
						<DropdownMenuItem
							className="gap-0 focus:bg-transparent text-xs text-neutral-11 p-3 py-1 hover:cursor-pointer ring-1 flex justify-start items-center"
							onClick={() => navigate({ to: '/projects' })}
						>
							<Button
								variant={'ghost'}
								className="p-0 gap-0 flex justify-start w-[85%] hover:bg-transparent"
							>
								<Avatar className="rounded-[100%] w-6 h-6 text-xs mr-2">
									<AvatarFallback className="bg-new.primary-600 text-white-100 text-[10px]">
										{transform.getInitials(org.name.split(' '))}
									</AvatarFallback>
								</Avatar>
								<span className="text-xs text-start block truncate w-3/4">
									{org.name}
								</span>
							</Button>
							<a
								href={`/settings`}
								className="block p-2 bg-new.primary-25 rounded-8px transition-colors"
							>
								<SettingsIcon size={18} className="stroke-neutral-9" />
							</a>
						</DropdownMenuItem>
					</DropdownMenuGroup>

					<DropdownMenuSeparator />

					<DropdownMenuGroup>
						<ul>
							{paginatedOrgs.content
								.filter(_ => _.uid != org.uid)
								.toSorted((orgA, orgB) => {
									if (orgA.name < orgB.name) return -1;
									if (orgA.name > orgB.name) return 1;
									return 0;
								})
								.map(_org => {
									return (
										<DropdownMenuItem
											key={_org.uid}
											className="gap-0 text-xs text-neutral-11 p-3 py-1 hover:cursor-pointer flex justify-start items-center hover:bg-neutral-3"
											onClick={() => reloadProjects(_org)}
										>
											<Button
												variant={'ghost'}
												className="p-0 gap-0 flex justify-start w-[85%] hover:bg-transparent"
											>
												<Avatar className="rounded-[100%] w-6 h-6 mr-2">
													<AvatarFallback className="bg-new.primary-600 text-white-100 text-[10px]">
														{transform.getInitials(_org.name.split(' '))}
													</AvatarFallback>
												</Avatar>
												<span className="text-xs text-start block truncate w-3/4">
													{_org.name}
												</span>
											</Button>
										</DropdownMenuItem>
									);
								})}
						</ul>
					</DropdownMenuGroup>

					<DropdownMenuSeparator />

					<CreateOrganisationDialog
						trigger={
							<DropdownMenuItem
								disabled={!canCreateOrg}
								onClick={e => {
									e.preventDefault();
									setIsDialogOpen(isOpen => !isOpen);
								}}
								className="flex justify-center items-center hover:cursor-pointer hover:bg-neutral-3 py-3"
							>
								<div className="flex items-center justify-center">
									<CirclePlusIcon
										className="stroke-new.primary-400 mr-2"
										size={20}
									/>
									<span className="block text-new.primary-400 text-xs ">
										Add {paginatedOrgs.content.length == 0 ? 'an' : 'another'}{' '}
										organisation
									</span>
								</div>
								{/* TODO add tooltip here for when button is disabled */}
							</DropdownMenuItem>
						}
						isDialogOpen={isDialogOpen}
						onOrgCreated={reloadOrganisations}
						setIsDialogOpen={setIsDialogOpen}
					/>
				</DropdownMenuContent>
			</DropdownMenu>
		</li>
	);
}

function HeaderRight() {
	return (
		<nav>
			<ul className="flex items-center justify-end">
				<HeaderRightOrganisation />
				<HeaderRightProfile />
			</ul>
		</nav>
	);
}

export function DashboardHeader(props: { showToggleSidebarButton: boolean }) {
	const { toggleSidebar } = useSidebar();

	return (
		<header className="sticky top-0 z-50 border-b bg-background">
			<div className="flex w-full px-6 items-center justify-between mx-auto">
				<div className="flex items-center justify-between py-3 h-[60px] w-[17rem]">
					<a href="/" className="inline-block" rel="noreferrer">
						<img src={convoyLogo} alt="Convoy" className="w-[100px]" />
					</a>
					{props.showToggleSidebarButton ? (
						<div className="flex items-center ">
							<Button
								className="h-8 w-8"
								variant="ghost"
								size="icon"
								onClick={toggleSidebar}
								title="CTRL + B"
							>
								<SidebarIcon />
							</Button>
						</div>
					) : null}
				</div>
				<HeaderRight />
			</div>
		</header>
	);
}

const FormSchema = z.object({
	project: z.string({
		required_error: 'Please select a project.',
	}),
});

function ProjectsList() {
	const [isPopoverOpen, setIsPopoverOpen] = useState(false);
	const { project, projects, setProject } = useProjectStore();

	const form = useForm<z.infer<typeof FormSchema>>({
		resolver: zodResolver(FormSchema),
	});

	return (
		<>
			{project ? (
				<Form {...form}>
					<form>
						<FormField
							control={form.control}
							name="project"
							render={({ field }) => (
								<FormItem className="flex flex-col z-10">
									<Popover open={isPopoverOpen} onOpenChange={setIsPopoverOpen}>
										<PopoverTrigger asChild>
											<FormControl>
												<Button
													variant="ghost"
													role="combobox"
													aria-expanded={isPopoverOpen}
													className="px-2 flex justify-stretch items-center hover:bg-neutral-3"
												>
													{project ? (
														<div className="flex items-center grow font-semibold">
															<svg
																width="16"
																height="16"
																className={cn(
																	'fill-primary-100 stroke-primary-100 mr-1',
																	project.type == 'incoming'
																		? 'rotate-180'
																		: '',
																)}
															>
																<use xlinkHref="#top-right-icon"></use>
															</svg>
															{transform.truncateProjectName(project.name)}
														</div>
													) : (
														<div className="flex items-center grow">
															<span className="inline-block pl-[18px]"></span>
															Select project...
														</div>
													)}

													<ChevronsUpDown className="opacity-50" />
												</Button>
											</FormControl>
										</PopoverTrigger>
										<PopoverContent className="w-full mt-1 p-0 z-10 rounded-md border bg-popover text-popover-foreground shadow-md outline-none data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2">
											<Command>
												<CommandInput
													placeholder={'Filter projects'}
													className=""
													onInput={e => {
														form.setValue(
															'project',
															(e.target as HTMLInputElement).value,
														);
													}}
												/>
												<CommandList>
													<CommandEmpty>
														{projects.length
															? `No projects found for '${field.value}'`
															: 'No projects to filter'}
													</CommandEmpty>
													<CommandGroup>
														{projects.length > 0 &&
															projects.map(p => (
																<CommandItem
																	className="hover:cursor-pointer flex"
																	key={p.uid}
																	value={p.name}
																	onSelect={() => {
																		setProject(p);
																		setIsPopoverOpen(false);
																	}}
																>
																	<div className="flex items-center grow">
																		<svg
																			width="16"
																			height="16"
																			className={cn(
																				'fill-primary-100 stroke-primary-100 mr-1',
																				p.type == 'incoming'
																					? 'rotate-180'
																					: '',
																			)}
																		>
																			<use xlinkHref="#top-right-icon"></use>
																		</svg>
																		{transform.truncateProjectName(p.name)}
																	</div>

																	<Check
																		className={cn(
																			'ml-auto',
																			project?.name === p.name
																				? 'opacity-100'
																				: 'opacity-0',
																		)}
																	/>
																</CommandItem>
															))}
													</CommandGroup>
												</CommandList>
											</Command>
										</PopoverContent>
									</Popover>
								</FormItem>
							)}
						/>
					</form>
				</Form>
			) : null}
		</>
	);
}

function ProjectLinks() {
	const { project } = useProjectStore();

	const links = [
		{
			name: 'Event Deliveries',
			route: '/',
		},
		{
			name: 'Sources',
			route: '/',
		},
		{
			name: 'Subscriptions',
			route: '/',
		},
		{
			name: 'Endpoints',
			route: '/',
		},
		{
			name: 'Events Log',
			route: '/',
		},
		{
			name: 'Meta Events',
			route: '/',
		},
		{
			name: 'Project Settings',
			route: `/projects/${project?.uid}/settings`,
		},
	];

	return (
		<>
			{project ? (
				<nav>
					<ul className="ml-5">
						{links.map(link => {
							return (
								<li key={link.name} className="mb-1">
									{/* TODO change to link route */}
									<Link
										to={link.route}
										className="flex hover:bg-neutral-3 py-2 pr-3 pl-2 rounded-sm"
										activeProps={{
											className: 'bg-neutral-4 hover:bg-neutral-4',
										}}
									>
										{link.name}
									</Link>
								</li>
							);
						})}
					</ul>
				</nav>
			) : null}
		</>
	);
}

function FooterLinks() {
	return (
		<nav className="w-full text-sm">
			<ul className="flex flex-col">
				<li>
					<a
						href="https://www.getconvoy.io/docs/legal/support-policy"
						rel="noreferrer"
						target="_blank"
						className="flex items-center justify-start w-full hover:bg-neutral-3 py-2 pr-3 pl-2 rounded-sm"
					>
						<HelpCircle className="mr-2" />
						<span>Help</span>
					</a>
				</li>
				<li>
					<a
						href="https://www.getconvoy.io/docs/home/quickstart"
						rel="noreferrer"
						target="_blank"
						className="flex items-center justify-start w-full hover:bg-neutral-3 py-2 pr-3 pl-2 rounded-sm"
					>
						<BookOpen className="mr-2" />
						<span>Documentation</span>
					</a>
				</li>
			</ul>
		</nav>
	);
}

export function DashboardSidebar({ ...props }: ComponentProps<typeof Sidebar>) {
	const navigate = useNavigate();
	const { org } = useOrganisationStore();
	const { licenses } = useLicenseStore();

	const [canCreateProject] = useState<boolean>(
		licenses.includes('CREATE_PROJECT'),
	);

	return (
		<aside>
			<Sidebar
				className="top-[--header-height] !h-[calc(100svh-var(--header-height))]"
				{...props}
			>
				<SidebarContent className="gap-0 mt-1">
					<TooltipProvider delayDuration={100}>
						<Tooltip>
							<TooltipTrigger asChild>
								<SidebarGroup>
									<SidebarGroupContent className="flex flex-col justify-center items-center">
										<Button
											onClick={() => navigate({ to: '/projects/new' })}
											disabled={!org || !canCreateProject}
											variant={'ghost'}
											className={cn(
												'w-full hover:bg-neutral-3 justify-start pl-7',
												org ? '' : 'blur-[1px]',
											)}
										>
											Create a new project
										</Button>
									</SidebarGroupContent>
								</SidebarGroup>
							</TooltipTrigger>
							{!org || !canCreateProject ? (
								<TooltipContent
									side="right"
									sideOffset={-48}
									className="bg-primary-100"
								>
									<p className="text-white-100 text-xs">
										{!org
											? 'An organisation is required to create projects on Convoy.'
											: 'Available on Business'}
									</p>
								</TooltipContent>
							) : null}
						</Tooltip>
					</TooltipProvider>

					<SidebarGroup>
						<SidebarGroupContent>
							<ProjectsList />
						</SidebarGroupContent>
					</SidebarGroup>

					<SidebarGroup>
						<SidebarGroupContent>
							<ProjectLinks />
						</SidebarGroupContent>
					</SidebarGroup>
				</SidebarContent>
				<SidebarFooter>
					<FooterLinks />
				</SidebarFooter>
			</Sidebar>
		</aside>
	);
}

export function DashboardLayout(props: {
	children: ReactNode;
	showSidebar: boolean;
}) {
	return (
		<div className="[--header-height:calc(theme(spacing.14))]">
			<SidebarProvider className="flex flex-col">
				<DashboardHeader showToggleSidebarButton={props.showSidebar} />
				<div className="flex h-full">
					{props.showSidebar ? <DashboardSidebar /> : null}
					<SidebarInset className="min-h-full">{props.children}</SidebarInset>
				</div>
			</SidebarProvider>
		</div>
	);
}
