import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { useState, useEffect } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';

import { BookOpen, Check, ChevronsUpDown, HelpCircle } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
	Sidebar,
	SidebarGroup,
	SidebarContent,
	SidebarFooter,
	SidebarGroupContent,
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
import * as transform from '@/lib/pipes';
import * as projectsService from '@/services/projects.service';
import * as organisationsService from '@/services/organisations.service';

import type { ComponentProps } from 'react';
import type { Project } from '@/models/project.model';

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

function ProjectLinks() {

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
			route: '/',
		},
	];

	const [currentProject] = useState(projectsService.getCachedProject());

	return (
		<>
			{currentProject ? (
				<nav>
					<ul className="ml-5">
						{links.map(link => {
							return (
								<li key={link.name}>
									{/* TODO change to link route */}
									<a
										href={link.route}
										className="flex hover:bg-neutral-3 py-2 pr-3 pl-2 rounded-sm"
									>
										{link.name}
									</a>
								</li>
							);
						})}
					</ul>
				</nav>
			) : null}
		</>
	);
}

const FormSchema = z.object({
	project: z.string({
		required_error: 'Please select a project.',
	}),
});

function ProjectsList() {
	const [isPopoverOpen, setIsPopoverOpen] = useState(false);
	const [projects, setProjects] = useState<Array<Project>>([]);
	const [selectedProject, setSelectedProject] = useState<Project | null>(null);

	useEffect(() => {
		projectsService
			.getProjects({ refresh: true })
			.then(data => {
				setProjects(data);
				setSelectedProject(data?.at(0) || null);
				projectsService.setCachedProject(data?.at(0) || null)
			})
			.catch(console.error);
	}, []);

	const form = useForm<z.infer<typeof FormSchema>>({
		resolver: zodResolver(FormSchema),
	});

	return (
		<>
			{selectedProject ? (
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
													{selectedProject ? (
														<div className="flex items-center grow font-bold">
															<svg
																width="16"
																height="16"
																className={cn(
																	'fill-primary-100 stroke-primary-100 mr-1',
																	selectedProject.type == 'incoming'
																		? 'rotate-180'
																		: '',
																)}
															>
																<use xlinkHref="#top-right-icon"></use>
															</svg>
															{transform.truncateProjectName(
																selectedProject.name,
															)}
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
													placeholder={'Filter projects...'}
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
																		setSelectedProject(p);
																		projectsService.setCachedProject(p)
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
																			selectedProject?.name === p.name
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

export function DashboardSidebar({ ...props }: ComponentProps<typeof Sidebar>) {
	const [cachedOrganisation] = useState(
		organisationsService.getCachedOrganisation(),
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
											disabled={!cachedOrganisation}
											variant={'ghost'}
											className={cn(
												'w-full hover:bg-neutral-3 ',
												cachedOrganisation ? '' : 'blur-[1px]',
											)}
										>
											Create a new project
										</Button>
									</SidebarGroupContent>
								</SidebarGroup>
							</TooltipTrigger>
							{!cachedOrganisation ? (
								<TooltipContent
									side="right"
									sideOffset={-48}
									className="bg-primary-100"
								>
									<p className="text-white-100 text-xs">
										An organization is required to create projects on Convoy.
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
