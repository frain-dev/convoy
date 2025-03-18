import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { useState, useCallback, useMemo } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { createFileRoute, useNavigate } from '@tanstack/react-router';

import {
	Bot,
	Home,
	Settings,
	User,
	Plus,
	X,
	PencilLine,
	Trash2,
} from 'lucide-react';

import {
	Form,
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormDescription,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import { Command as CommandPrimitive } from 'cmdk';
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTrigger,
	SheetTitle,
	SheetClose,
} from '@/components/ui/sheet';
import {
	Table,
	TableBody,
	TableCaption,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from '@/components/ui/table';
import {
	Dialog,
	DialogClose,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from '@/components/ui/dialog';
import {
	Accordion,
	AccordionContent,
	AccordionItem,
	AccordionTrigger,
} from '@/components/ui/accordion';
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from '@/components/ui/select';
import {
	Command,
	CommandGroup,
	CommandItem,
	CommandList,
} from '@/components/ui/command';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { DashboardLayout } from '@/components/dashboard';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';

import { cn } from '@/lib/utils';
import { toMMMDDYYYY } from '@/lib/pipes';
import { useProjectStore } from '@/store/index';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as authService from '@/services/auth.service';
import * as projectsService from '@/services/projects.service';
import { MetaEventTypes } from '@/models/project.model';

import type { KeyboardEvent } from 'react';
import type { EventType, MetaEventType, Project } from '@/models/project.model';

import warningAnimation from '../../../../assets/img/warning-animation.gif';
import eventTypesEmptyState from '../../../../assets/img/events-log-empty-state.png';

export const Route = createFileRoute('/projects_/$projectId/settings')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	async loader({ params }) {
		const project = await projectsService.getProject(params.projectId);
		// TODO handle error, and in other loaders too
		const canManageProject = await authService.ensureUserCanAccess(
			'Project Settings|MANAGE',
		);

		const { event_types } = await projectsService.getEventTypes(
			params.projectId,
		);

		return { project, canManageProject, eventTypes: event_types };
	},
	component: ProjectSettings,
});

const ProjectConfigFormSchema = z.object({
	name: z
		.string({
			required_error: 'Project name is required',
		})
		.min(1, 'Project name is required'),
	config: z
		.object({
			search_policy: z
				.object({
					isEnabled: z.boolean().optional(),
					search_policy: z
						.string()
						.optional()
						.transform(v => {
							if (!v?.length) return undefined;
							if (v.endsWith('h')) return v;
							return `${v}h`;
						}),
				})
				.transform(search => (search?.isEnabled ? search : undefined))
				.refine(
					search => {
						if (search?.isEnabled && !search.search_policy) return false;
						return true;
					},
					{
						message: 'Invalid Search Policy',
						path: ['search_policy'],
					},
				),
			strategy: z
				.object({
					isEnabled: z.boolean().optional(),
					duration: z
						.string()
						.optional()
						.transform(v => (!v?.length ? undefined : Number(v))),
					retry_count: z
						.string()
						.optional()
						.transform(v => (!v?.length ? undefined : Number(v))),
					type: z
						.enum(['linear', 'exponential', ''])
						.optional()
						.transform(v => (v?.length == 0 ? undefined : v)),
				})
				.optional()
				.transform(strategy => (strategy?.isEnabled ? strategy : undefined))
				.refine(
					strategy => {
						if (
							strategy?.isEnabled &&
							(!strategy?.duration || !strategy.retry_count || !strategy.type)
						)
							return false;

						return true;
					},
					strategy => {
						if (!strategy?.duration) {
							return {
								message: 'Invalid retry logic duration',
								path: ['duration'],
							};
						}
						if (!strategy?.type) {
							return {
								message: 'Invalid retry logic mechanism',
								path: ['type'],
							};
						}
						if (!strategy?.retry_count) {
							return {
								message: 'Invalid retry logic limit',
								path: ['retry_count'],
							};
						}
						return {
							message: '',
						};
					},
				),
			signature: z
				.object({
					isEnabled: z.boolean().optional(),
					header: z
						.string()
						.optional()
						.transform(v => (v?.length == 0 ? undefined : v)),
					encoding: z
						.enum(['hex', 'base64', ''])
						.optional()
						.transform(v => (v?.length == 0 ? undefined : v)),
					hash: z
						.enum(['SHA256', 'SHA512', ''])
						.optional()
						.transform(v => (v?.length == 0 ? undefined : v)),
				})
				.optional()
				.transform(sig => (sig?.isEnabled ? sig : undefined))
				.refine(
					sig => {
						if (sig?.isEnabled && (!sig.header || !sig.encoding || !sig.hash))
							return false;
						return true;
					},
					sig => {
						if (!sig?.header) {
							return { message: 'Invalid signature header', path: ['header'] };
						}
						if (!sig?.encoding) {
							return {
								message: 'Invalid signature encoding',
								path: ['encoding'],
							};
						}
						if (!sig?.hash) {
							return { message: 'Invalid signature hash', path: ['hash'] };
						}
						return { message: '' };
					},
				),
			ratelimit: z
				.object({
					isEnabled: z.boolean().optional(),
					count: z
						.string()
						.optional()
						.transform(v => (!v?.length ? undefined : Number(v))),
					duration: z
						.string()
						.optional()
						.transform(v => (!v?.length ? undefined : Number(v))),
				})
				.optional()
				.transform(ratelimit => (ratelimit?.isEnabled ? ratelimit : undefined))
				.refine(
					ratelimit => {
						const isInvalidRateLimitOpts =
							ratelimit?.isEnabled &&
							(!ratelimit?.count || !ratelimit?.duration);

						return !isInvalidRateLimitOpts;
					},
					ratelimit => {
						if (ratelimit?.count == undefined)
							return { message: 'Invalid rate limit count', path: ['count'] };

						if (ratelimit?.duration == undefined)
							return {
								message: 'Invalid rate limit duration',
								path: ['duration'],
							};

						return { message: '' };
					},
				),
		})
		.optional(),
});

function ProjectConfig(props: { project: Project; canManageProject: boolean }) {
	const [_project, set_Project] = useState(props.project);
	const { setProject, setProjects, projects } = useProjectStore();
	const [isUpdatingProject, setIsUpdatingProject] = useState(false);
	const [isDeletingProject, setIsDeletingProject] = useState(false);
	const navigate = useNavigate();

	const form = useForm<z.infer<typeof ProjectConfigFormSchema>>({
		resolver: zodResolver(ProjectConfigFormSchema),
		defaultValues: {
			name: _project.name,
			config: {
				search_policy: {
					isEnabled: !!_project.config.search_policy.length,
					search_policy: _project.config.search_policy.length
						? _project.config.search_policy.substring(
								0,
								_project.config.search_policy.length - 1,
							)
						: '',
				},
				ratelimit: {
					isEnabled: true,
					// @ts-expect-error this works as the input elements require this type
					count: `${_project.config.ratelimit.count}`,
					// @ts-expect-error this works as the input elements require this type
					duration: `${_project.config.ratelimit.duration}`,
				},
				strategy: {
					isEnabled: true,
					// @ts-expect-error this works as the input elements require this type
					duration: `${_project.config.strategy.duration}`,
					// @ts-expect-error this works as the input elements require this type
					retry_count: `${_project.config.strategy.retry_count}`,
					type: _project.config.strategy.type,
				},
				signature: {
					isEnabled: _project.type == 'outgoing',
					header: _project.config.signature.header,
					// @ts-expect-error a default value exists, even for incoming projects
					encoding: _project.config.signature.versions.at(
						_project.config.signature.versions.length - 1,
					)?.encoding,
					// @ts-expect-error a default value exists, even for incoming projects
					hash: _project.config.signature.versions.at(
						_project.config.signature.versions.length - 1,
					)?.hash,
				},
			},
		},
		mode: 'onTouched',
	});
	const shouldShowRetryConfig = form.watch('config.strategy.isEnabled');
	const shouldShowRateLimit = form.watch('config.ratelimit.isEnabled');
	const shouldShowSearchPolicy = form.watch('config.search_policy.isEnabled');
	const shouldShowSigFormat = form.watch('config.signature.isEnabled');

	async function reloadProjects(p: Project) {
		const projects = await projectsService.getProjects();
		setProjects(projects);
		setProject(p);
		set_Project(p);
	}

	async function updateProject(
		values: z.infer<typeof ProjectConfigFormSchema>,
	) {
		setIsUpdatingProject(true);
		let payload = {
			name: values.name,
			config: {} as z.infer<typeof ProjectConfigFormSchema>['config'],
		};

		// @ts-expect-error it works. source: trust the code
		payload = Object.entries(values.config).reduce((acc, [key, val]) => {
			if (!values.config) return acc;

			// @ts-expect-error it works. source: trust the code
			if (!values.config[key] || values.config[key]['isEnabled'] === false) {
				return acc;
			}

			// @ts-expect-error it works. source: trust me
			delete values.config[key]['isEnabled'];
			return {
				...acc,
				config: {
					...acc.config,
					[key]: val,
				},
			};
		}, payload);

		try {
			const updated = await projectsService.updateProject({
				name: payload.name,
				type: _project.type,
				config: {
					...payload.config,
					search_policy: payload.config?.search_policy
						?.search_policy as `{string}h`,
					// @ts-expect-error it has to be this way for the API
					signature: payload.config?.signature
						? {
								header: payload.config.signature.header,
								versions: [
									{
										hash: payload.config.signature.hash,
										encoding: payload.config.signature.encoding,
									},
								],
							}
						: _project.config.signature,
					disable_endpoint: _project.config.disable_endpoint,
					multiple_endpoint_subscriptions:
						_project.config.multiple_endpoint_subscriptions,
					ssl: _project.config.ssl,
					meta_event: _project.config.meta_event,
				},
			});

			await reloadProjects(updated);
		} catch (error) {
			// TODO: notify UI of error
			console.error(error);
		} finally {
			setIsUpdatingProject(false);
		}
	}

	async function deleteProject() {
		setIsDeletingProject(true);
		try {
			await projectsService.deleteProject(_project.uid);
			setProjects(projects.filter(p => p.uid != _project.uid));
			setProject(projects.at(1) || null);
			navigate({ to: '/projects' });
		} catch (error) {
			// TODO notify UI
			console.error(error);
		} finally {
			setIsDeletingProject(false);
			// TODO notify UI
		}
	}

	return (
		<section className="flex flex-col">
			<h1>Project Configuration</h1>
			<div className="w-full">
				<Form {...form}>
					<form
						onSubmit={(...args) =>
							void form.handleSubmit(updateProject)(...args)
						}
					>
						<div className="flex flex-col w-full">
							<FormField
								control={form.control}
								name="name"
								render={({ field, fieldState }) => (
									<FormItem className="w-full relative mb-6 block">
										<div className="w-full mb-2 flex items-center justify-between">
											<FormLabel className="text-xs/5 text-neutral-9">
												Project name
											</FormLabel>
										</div>
										<FormControl>
											<Input
												autoComplete="on"
												type="text"
												className={cn(
													'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
													fieldState.error
														? 'border-destructive focus-visible:ring-0 hover:border-destructive'
														: ' hover:border-new.primary-100 focus:border-new.primary-300',
												)}
												{...field}
											/>
										</FormControl>
										<FormMessageWithErrorIcon />
									</FormItem>
								)}
							/>

							<div className="flex justify-between gap-4 my-2 w-[90%]">
								<label className="flex items-center gap-2 cursor-pointer">
									{/* TODO you may want to make this label into a component */}
									<FormField
										control={form.control}
										name="config.strategy.isEnabled"
										render={({ field }) => (
											<FormItem>
												<FormControl className="relative">
													<div className="relative">
														<input
															type="checkbox"
															className=" peer
    appearance-none w-[14px] h-[14px] border-[1px] border-new.primary-300 rounded-sm bg-white-100
    mt-1 shrink-0 checked:bg-new.primary-300
     checked:border-0 cursor-pointer"
															checked={field.value}
															onChange={field.onChange}
														/>
														<svg
															className="
      absolute
      w-3 h-3 mt-1
      hidden peer-checked:block top-[0.5px] right-[1px]"
															xmlns="http://www.w3.org/2000/svg"
															viewBox="0 0 24 24"
															fill="none"
															stroke="white"
															strokeWidth="4"
															strokeLinecap="round"
															strokeLinejoin="round"
														>
															<polyline points="20 6 9 17 4 12"></polyline>
														</svg>
													</div>
												</FormControl>
											</FormItem>
										)}
									/>
									<span className="block text-neutral-9 text-xs">
										Retry Configuration
									</span>
								</label>

								<label className="flex items-center gap-2 cursor-pointer">
									<FormField
										control={form.control}
										name="config.ratelimit.isEnabled"
										render={({ field }) => (
											<FormItem>
												<FormControl>
													<div className="relative">
														<input
															type="checkbox"
															className=" peer
    appearance-none w-[14px] h-[14px] border-[1px] border-new.primary-300 rounded-sm bg-white-100
    mt-1 shrink-0 checked:bg-new.primary-300
     checked:border-0 cursor-pointer"
															checked={field.value}
															onChange={field.onChange}
														/>
														<svg
															className="
      absolute
      w-3 h-3 mt-1
      hidden peer-checked:block top-[0.5px] right-[1px]"
															xmlns="http://www.w3.org/2000/svg"
															viewBox="0 0 24 24"
															fill="none"
															stroke="white"
															strokeWidth="4"
															strokeLinecap="round"
															strokeLinejoin="round"
														>
															<polyline points="20 6 9 17 4 12"></polyline>
														</svg>
													</div>
												</FormControl>
											</FormItem>
										)}
									/>
									<span className="block text-neutral-9 text-xs">
										Rate Limit
									</span>
								</label>

								<label className="flex items-center gap-2 cursor-pointer">
									<FormField
										control={form.control}
										name="config.search_policy.isEnabled"
										render={({ field }) => (
											<FormItem>
												<FormControl>
													<div className="relative">
														<input
															type="checkbox"
															className=" peer
    appearance-none w-[14px] h-[14px] border-[1px] border-new.primary-300 rounded-sm bg-white-100
    mt-1 shrink-0 checked:bg-new.primary-300
     checked:border-0 cursor-pointer"
															checked={field.value}
															onChange={field.onChange}
														/>
														<svg
															className="
      absolute
      w-3 h-3 mt-1
      hidden peer-checked:block top-[0.5px] right-[1px]"
															xmlns="http://www.w3.org/2000/svg"
															viewBox="0 0 24 24"
															fill="none"
															stroke="white"
															strokeWidth="4"
															strokeLinecap="round"
															strokeLinejoin="round"
														>
															<polyline points="20 6 9 17 4 12"></polyline>
														</svg>
													</div>
												</FormControl>
											</FormItem>
										)}
									/>
									<span className="block text-neutral-9 text-xs">
										Search Policy
									</span>
								</label>

								<label className="flex items-center gap-2 cursor-pointer">
									<FormField
										control={form.control}
										name="config.signature.isEnabled"
										render={({ field }) => (
											<FormItem>
												<FormControl>
													<div className="relative">
														<input
															disabled={_project.type !== 'outgoing'}
															type="checkbox"
															className=" peer
    appearance-none w-[14px] h-[14px] border-[1px] border-new.primary-300 rounded-sm bg-white-100
    mt-1 shrink-0 checked:bg-new.primary-300
     checked:border-0 cursor-pointer disabled:opacity-50"
															checked={field.value}
															onChange={field.onChange}
														/>
														<svg
															className="
      absolute
      w-3 h-3 mt-1
      hidden peer-checked:block top-[0.5px] right-[1px]"
															xmlns="http://www.w3.org/2000/svg"
															viewBox="0 0 24 24"
															fill="none"
															stroke="white"
															strokeWidth="4"
															strokeLinecap="round"
															strokeLinejoin="round"
														>
															<polyline points="20 6 9 17 4 12"></polyline>
														</svg>
													</div>
												</FormControl>
											</FormItem>
										)}
									/>
									<span
										className={cn(
											'block text-neutral-9 text-xs',
											_project.type != 'outgoing' ? 'opacity-50' : '',
										)}
									>
										Signature Format
									</span>
								</label>
							</div>
						</div>

						<div>
							<Accordion
								type="multiple"
								className="w-full transition-all duration-300 mb-6"
							>
								{shouldShowRetryConfig ? (
									<AccordionItem value="retry-config">
										<AccordionTrigger className="py-2 text-xs text-neutral-9 hover:no-underline">
											Retry Configuration
										</AccordionTrigger>
										<AccordionContent>
											<h4 className="font-semibold text-neutral-11 text-xs mb-2">
												Retry Logic (add tooltip here)
											</h4>
											<div className="w-6/12">
												<FormField
													control={form.control}
													name="config.strategy.type"
													render={({ field }) => (
														<FormItem className="w-full relative mb-6 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Mechanism
																</FormLabel>
															</div>
															<Select
																onValueChange={field.onChange}
																defaultValue={field.value}
															>
																<FormControl>
																	<SelectTrigger>
																		<SelectValue defaultValue={field.value} />
																	</SelectTrigger>
																</FormControl>
																<SelectContent>
																	<SelectItem
																		value="linear"
																		className="cursor-pointer"
																	>
																		Linear time retry
																	</SelectItem>
																	<SelectItem
																		value="exponential"
																		className="cursor-pointer"
																	>
																		Exponential time backoff
																	</SelectItem>
																</SelectContent>
															</Select>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="config.strategy.duration"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative mb-2 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel
																	className="text-xs/5 text-neutral-9"
																	htmlFor="config_strategy_duration"
																>
																	Duration
																</FormLabel>
															</div>
															<FormControl className="w-full relative">
																<div className="relative">
																	<Input
																		id="config_strategy_duration"
																		inputMode="numeric"
																		pattern="\d*"
																		type="number"
																		min={0}
																		className={cn(
																			'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																			fieldState.error
																				? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																				: 'hover:border-new.primary-100 focus:border-new.primary-300',
																		)}
																		placeholder="e.g 30"
																		{...field}
																	/>
																	<span className="absolute right-[1%] top-4 h-full px-3 text-xs text-neutral-9">
																		sec
																	</span>
																</div>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="config.strategy.retry_count"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative mb-6 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Limit
																</FormLabel>
															</div>
															<FormControl>
																<Input
																	type="number"
																	inputMode="numeric"
																	pattern="\d*"
																	min={0}
																	placeholder="e.g 5"
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>
										</AccordionContent>
									</AccordionItem>
								) : null}

								{shouldShowRateLimit ? (
									<AccordionItem value="rate-limit">
										<AccordionTrigger className="py-2 text-xs text-neutral-9 hover:no-underline">
											Rate Limit
										</AccordionTrigger>
										<AccordionContent>
											<h4 className="font-semibold text-neutral-11 text-xs mb-2">
												Rate Limit Parameters (add tooltip here)
											</h4>
											<div className="w-6/12">
												<FormField
													control={form.control}
													name="config.ratelimit.duration"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative mb-2 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel
																	className="text-xs/5 text-neutral-9"
																	htmlFor="config_ratelimit_duration"
																>
																	Duration
																</FormLabel>
															</div>
															<FormControl className="w-full relative">
																<div className="relative">
																	<Input
																		id="config_ratelimit_duration"
																		type="number"
																		inputMode="numeric"
																		pattern="\d*"
																		min={0}
																		className={cn(
																			'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																			fieldState.error
																				? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																				: 'hover:border-new.primary-100 focus:border-new.primary-300',
																		)}
																		placeholder="e.g 50"
																		{...field}
																	/>
																	<span className="absolute right-[1%] top-4 h-full px-3 text-xs text-neutral-9">
																		sec
																	</span>
																</div>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="config.ratelimit.count"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative mb-6 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Limit
																</FormLabel>
															</div>
															<FormControl>
																<Input
																	type="number"
																	inputMode="numeric"
																	pattern="\d*"
																	min={1}
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																	placeholder="e.g 10"
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>
										</AccordionContent>
									</AccordionItem>
								) : null}

								{shouldShowSearchPolicy ? (
									<AccordionItem value="search-policy">
										<AccordionTrigger className="py-2 text-xs text-neutral-9 hover:no-underline">
											Search Policy
										</AccordionTrigger>
										<AccordionContent>
											<h4 className="font-semibold text-neutral-11 text-xs mb-2">
												{/* TODO don't forget to add tooltip here */}
												Search Period (add tooltip here)
											</h4>
											<div className="grid">
												<FormField
													control={form.control}
													name="config.search_policy.search_policy"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative mb-2 block">
															<FormControl className="w-full relative">
																<div className="relative">
																	<Input
																		id="config_search_policy"
																		type="number"
																		inputMode="numeric"
																		pattern="\d*"
																		min={0}
																		className={cn(
																			'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																			fieldState.error
																				? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																				: 'hover:border-new.primary-100 focus:border-new.primary-300',
																		)}
																		placeholder="e.g 720"
																		{...field}
																	/>
																	<span className="absolute right-[1%] top-4 h-full px-3 text-xs text-neutral-9">
																		hour(s)
																	</span>
																</div>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>
										</AccordionContent>
									</AccordionItem>
								) : null}

								{_project.type == 'outgoing' && shouldShowSigFormat ? (
									<AccordionItem value="signature-format">
										<AccordionTrigger className="py-2 text-xs text-neutral-9 hover:no-underline">
											Signature Format
										</AccordionTrigger>
										<AccordionContent>
											<h4 className="font-semibold text-neutral-11 text-xs mb-2">
												Signature Details (add tooltip here)
											</h4>
											<div className="w-6/12">
												<FormField
													control={form.control}
													name="config.signature.header"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative mb-6 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Header
																</FormLabel>
															</div>
															<FormControl>
																<Input
																	type="text"
																	placeholder="e.g X-Convoy-Signature"
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="config.signature.encoding"
													render={({ field }) => (
														<FormItem className="w-full relative mb-6 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Encoding
																</FormLabel>
															</div>
															<Select
																onValueChange={field.onChange}
																defaultValue={field.value}
															>
																<FormControl>
																	<SelectTrigger>
																		<SelectValue />
																	</SelectTrigger>
																</FormControl>
																<SelectContent>
																	<SelectItem
																		value="base64"
																		className="cursor-pointer"
																	>
																		base64
																	</SelectItem>
																	<SelectItem
																		value="hex"
																		className="cursor-pointer"
																	>
																		hex
																	</SelectItem>
																</SelectContent>
															</Select>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="config.signature.hash"
													render={({ field }) => (
														<FormItem className="w-full relative mb-6 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Hash
																</FormLabel>
															</div>
															<Select
																onValueChange={field.onChange}
																defaultValue={field.value}
															>
																<FormControl>
																	<SelectTrigger>
																		<SelectValue />
																	</SelectTrigger>
																</FormControl>
																<SelectContent>
																	<SelectItem
																		value="SHA256"
																		className="cursor-pointer"
																	>
																		SHA256
																	</SelectItem>
																	<SelectItem
																		value="SHA512"
																		className="cursor-pointer"
																	>
																		SHA512
																	</SelectItem>
																</SelectContent>
															</Select>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>
										</AccordionContent>
									</AccordionItem>
								) : null}
							</Accordion>
						</div>

						<div className="flex justify-end">
							<Button
								disabled={
									!props.canManageProject ||
									!form.formState.isValid ||
									isUpdatingProject
								}
								variant="ghost"
								className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
							>
								Save Changes
							</Button>
						</div>
					</form>
				</Form>
			</div>

			<hr className="my-12 border-neutral-5" />

			<section className="bg-destructive/5 border-destructive/30 border p-6 rounded-8px flex flex-col items-start justify-center">
				<h2 className="text-destructive font-semibold text-lg mb-5">
					Danger Zone
				</h2>
				<div className="text-sm mb-8">
					<p className="mb-3">
						Deleting this project will delete all of it&apos;s data including
						events, apps, subscriptions and configurations.
					</p>
					<b className="font-semibold ">
						Are you sure you want to delete this project?
					</b>
				</div>
				<Dialog>
					<DialogTrigger asChild>
						<Button
							disabled={isDeletingProject || !props.canManageProject}
							size="sm"
							variant="ghost"
							className="px-4 py-2 text-xs bg-destructive  hover:bg-destructive hover:text-white-100 flex items-center"
						>
							<svg width="18" height="18" className="fill-white-100">
								<use xlinkHref="#delete-icon"></use>
							</svg>
							<p className="text-white-100">Delete Project</p>
						</Button>
					</DialogTrigger>
					<DialogContent className="sm:max-w-[432px] rounded-lg">
						<DialogHeader>
							<DialogTitle className="flex justify-center items-center">
								<img src={warningAnimation} alt="warning" className="w-24" />
							</DialogTitle>
							<DialogDescription className="flex justify-center items-center font-medium text-new.black text-sm">
								Are you sure you want to deactivate “{_project?.name}”?
							</DialogDescription>
						</DialogHeader>
						<div className="flex flex-col items-center space-y-4">
							<p className="text-xs text-neutral-11">
								This action is irreversible.
							</p>
							<DialogClose asChild>
								<Button
									onClick={deleteProject}
									type="submit"
									size="sm"
									className="bg-destructive text-white-100 hover:bg-destructive hover:text-white-100"
								>
									Yes. Deactivate
								</Button>
							</DialogClose>
						</div>
						<DialogFooter className="flex justify-center items-center">
							<DialogClose asChild>
								<Button
									type="button"
									variant="ghost"
									className="bg-transparent hover:bg-transparent text-xs text-neutral-11 hover:text-neutral-11 font-semibold"
								>
									No. Cancel
								</Button>
							</DialogClose>
						</DialogFooter>
					</DialogContent>
				</Dialog>
			</section>
		</section>
	);
}

function groupItemsByDate<T>(
	items: Array<T & { created_at: string }>,
	sortOrder: 'desc' | 'asc' = 'desc',
) {
	const groupsObj = Object.groupBy(items, ({ created_at }) =>
		toMMMDDYYYY(created_at),
	);

	const sortedGroup = new Map<string, typeof items>();

	Object.keys(groupsObj)
		.sort((dateA, dateB) => {
			if (sortOrder == 'desc') {
				return Number(new Date(dateB)) - Number(new Date(dateA));
			}
			return Number(new Date(dateA)) - Number(new Date(dateB));
		})
		.reduce((acc, dateKey) => {
			return acc.set(dateKey, groupsObj[dateKey] as typeof items);
		}, sortedGroup);

	return sortedGroup;
}

const NewSignatureFormSchema = z.object({
	encoding: z
		.enum(['hex', 'base64', ''])
		.optional()
		.transform(v => (v?.length == 0 ? undefined : v))
		.refine(
			v => {
				if (!v) return false;
				return true;
			},
			{
				message: 'Select encoding type',
			},
		),
	hash: z
		.enum(['SHA256', 'SHA512', ''])
		.optional()
		.transform(v => (v?.length == 0 ? undefined : v))
		.refine(
			v => {
				if (!v) return false;
				return true;
			},
			{
				message: 'Please select hash',
			},
		),
});

function SignatureHistoryConfig(props: {
	project: Project;
	canManageProject: boolean;
}) {
	const { project } = props;
	const [isAddingVersion, setIsAddingVersion] = useState(false);
	const { setProjects, projects, setProject } = useProjectStore();
	// TODO update UI on new project added

	const form = useForm<z.infer<typeof NewSignatureFormSchema>>({
		resolver: zodResolver(NewSignatureFormSchema),
		defaultValues: {
			encoding: '',
			hash: '',
		},
		mode: 'onTouched',
	});

	async function addSignatureVersion(
		version: z.infer<typeof NewSignatureFormSchema>,
	) {
		try {
			setIsAddingVersion(true);
			const updated = await projectsService.updateProject({
				...project,
				config: {
					...project.config,
					signature: {
						...project.config.signature,
						versions: [
							// @ts-expect-error this works
							...project.config.signature.versions.map(v => ({
								encoding: v.encoding,
								hash: v.hash,
							})),
							// @ts-expect-error this works
							version,
						],
					},
				},
			});
			setProjects(projects.map(p => (p.uid == updated.uid ? updated : p)));
			setProject(updated);
		} catch (err) {
			console.error(err);
		} finally {
			setIsAddingVersion(false);
		}
	}

	if (project.type == 'incoming') return null;

	return (
		<section>
			<div className="flex justify-between items-center mb-6">
				<h1 className="font-bold py-2">Project Signature History</h1>
				<Sheet>
					<SheetTrigger asChild>
						<Button
							disabled={!props.canManageProject}
							size={'sm'}
							variant="ghost"
							className="hover:bg-new.primary-400 px-3 text-xs hover:text-white-100 bg-new.primary-400 flex justify-between items-center"
						>
							<Plus className="stroke-white-100" />
							<p className=" text-white-100">Signature</p>
						</Button>
					</SheetTrigger>
					<SheetContent className="w-[400px] px-0">
						<SheetHeader className="text-start px-4">
							<SheetTitle>New Signature</SheetTitle>
							<SheetDescription className="sr-only">
								Add a new signature
							</SheetDescription>
						</SheetHeader>
						<hr className="my-6 border-neutral-5" />
						<div className="px-4">
							<Form {...form}>
								<form
									onSubmit={(...args) =>
										void form.handleSubmit(addSignatureVersion)(...args)
									}
								>
									<FormField
										control={form.control}
										name="encoding"
										render={({ field }) => (
											<FormItem className="w-full relative mb-6 block">
												<div className="w-full mb-2 flex items-center justify-between">
													<FormLabel className="text-xs/5 text-neutral-9">
														Encoding
													</FormLabel>
												</div>
												<Select
													onValueChange={field.onChange}
													defaultValue={field.value}
												>
													<FormControl>
														<SelectTrigger>
															<SelectValue />
														</SelectTrigger>
													</FormControl>
													<SelectContent>
														<SelectItem
															value="base64"
															className="cursor-pointer"
														>
															base64
														</SelectItem>
														<SelectItem value="hex" className="cursor-pointer">
															hex
														</SelectItem>
													</SelectContent>
												</Select>
												<FormMessageWithErrorIcon />
											</FormItem>
										)}
									/>

									<FormField
										control={form.control}
										name="hash"
										render={({ field }) => (
											<FormItem className="w-full relative mb-6 block">
												<div className="w-full mb-2 flex items-center justify-between">
													<FormLabel className="text-xs/5 text-neutral-9">
														Hash
													</FormLabel>
												</div>
												<Select
													onValueChange={field.onChange}
													defaultValue={field.value}
												>
													<FormControl>
														<SelectTrigger>
															<SelectValue />
														</SelectTrigger>
													</FormControl>
													<SelectContent>
														<SelectItem
															value="SHA256"
															className="cursor-pointer"
														>
															SHA256
														</SelectItem>
														<SelectItem
															value="SHA512"
															className="cursor-pointer"
														>
															SHA512
														</SelectItem>
													</SelectContent>
												</Select>
												<FormMessageWithErrorIcon />
											</FormItem>
										)}
									/>

									<div className="flex justify-end items-center gap-x-4">
										<SheetClose asChild>
											<Button
												variant="ghost"
												className="hover:bg-white-100 text-destructive hover:text-destructive border border-destructive text-xs hover:destructive"
											>
												Discard
											</Button>
										</SheetClose>

										<Button
											type="submit"
											disabled={
												!props.canManageProject ||
												!form.formState.isValid ||
												isAddingVersion
											}
											variant="ghost"
											className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
										>
											Create
										</Button>
									</div>
								</form>
							</Form>
						</div>
					</SheetContent>
				</Sheet>
			</div>

			<div>
				<Table>
					<TableCaption className="sr-only">
						{project.name} signature history
					</TableCaption>
					<TableHeader>
						<TableRow className="">
							<TableHead className=" uppercase text-new.black font-medium text-xs">
								header
							</TableHead>
							<TableHead className="uppercase text-new.black font-medium text-xs">
								version
							</TableHead>
							<TableHead className="uppercase text-new.black font-medium text-xs">
								hash
							</TableHead>
							<TableHead className="uppercase text-new.black font-medium text-xs">
								encoding
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{Array.from(
							groupItemsByDate(project.config.signature.versions),
						).map(([dateKey, sigs]) => {
							const val = [
								<TableRow
									key={dateKey}
									className="border-new.primary-25 border-t border-b-0 hover:bg-transparent"
								>
									<TableCell className="font-medium text-neutral-8">
										{dateKey}
									</TableCell>
									<TableCell className="font-medium"></TableCell>
									<TableCell className="font-medium"></TableCell>
									<TableCell className="font-medium"></TableCell>
								</TableRow>,
							].concat(
								sigs.map((sig, i) => (
									<TableRow
										key={sig.uid}
										className="duration-300 hover:bg-new.primary-25 transition-all py-3"
									>
										<TableCell className="font-medium">
											{project.config.signature.header}
										</TableCell>
										<TableCell className="font-medium">v{i + 1}</TableCell>
										<TableCell className="font-medium">{sig.hash}</TableCell>
										<TableCell className="font-medium">
											{sig.encoding}
										</TableCell>
									</TableRow>
								)),
							);

							return val;
						})}
					</TableBody>
				</Table>
			</div>
		</section>
	);
}

const EndpointsConfigFormSchema = z.object({
	disable_endpoint: z.boolean(),
	enforce_secure_endpoints: z.boolean(),
});

function EndpointsConfig(props: {
	project: Project;
	canManageProject: boolean;
}) {
	const { project, canManageProject } = props;
	const [isUpdatingDisableEndpoint, setIsUpdatingDisableEndpoint] =
		useState(false);
	const [
		isUpdatingEnforceSecureEndpoints,
		setIsUpdatingEnforceSecureEndpoints,
	] = useState(false);
	const { setProject, setProjects, projects } = useProjectStore();

	const form = useForm<z.infer<typeof EndpointsConfigFormSchema>>({
		resolver: zodResolver(EndpointsConfigFormSchema),
		defaultValues: {
			disable_endpoint: project.config.disable_endpoint,
			enforce_secure_endpoints: project.config.ssl.enforce_secure_endpoints,
		},
	});

	async function updateDisableEndpoint(value: boolean) {
		setIsUpdatingDisableEndpoint(true);
		try {
			project.config.disable_endpoint = value;
			// @ts-expect-error this works
			const updated = await projectsService.updateProject(project);
			setProjects(projects.map(p => (p.uid == updated.uid ? updated : p)));
			setProject(updated);
		} catch (error) {
			console.error(error);
		} finally {
			setIsUpdatingDisableEndpoint(false);
		}
	}

	async function updateEnforceSecureEndpoints(value: boolean) {
		setIsUpdatingEnforceSecureEndpoints(true);
		try {
			project.config.ssl.enforce_secure_endpoints = value;
			// @ts-expect-error this works
			const updated = await projectsService.updateProject(project);
			setProjects(projects.map(p => (p.uid == updated.uid ? updated : p)));
			setProject(updated);
		} catch (error) {
			console.error(error);
		} finally {
			setIsUpdatingEnforceSecureEndpoints(false);
		}
	}

	return (
		<section className="flex flex-col gap-4">
			<h1 className="font-bold">Endpoint Configurations</h1>

			<Form {...form}>
				<form>
					<div>
						<div className="space-y-4">
							<FormField
								control={form.control}
								name="disable_endpoint"
								render={({ field }) => (
									<FormItem className="flex flex-row items-center justify-between rounded-lg border p-3">
										<div className="space-y-0.5 hover:cursor-pointer">
											<FormLabel className="font-semibold text-xs">
												Disable Failing Endpoint
												<FormDescription className="max-w-prose font-normal text-xs/5 hover:cursor-pointer">
													Toggling this configuration on will automatically
													disable all endpoints in this project with failure
													response until requests to them are manually retried
												</FormDescription>
											</FormLabel>
										</div>
										<FormControl>
											<Switch
												disabled={
													!canManageProject || isUpdatingDisableEndpoint
												}
												checked={field.value}
												onCheckedChange={async e => {
													field.onChange(e);
													await updateDisableEndpoint(!field.value);
												}}
											/>
										</FormControl>
									</FormItem>
								)}
							/>

							<FormField
								control={form.control}
								name="enforce_secure_endpoints"
								render={({ field }) => (
									<FormItem className="flex flex-row items-center justify-between rounded-lg border p-3">
										<div className="space-y-0.5 hover:cursor-pointer">
											<FormLabel className="font-semibold text-xs">
												Allow Only HTTPS Secure Endpoints
												<FormDescription className="max-w-prose font-normal text-xs/5">
													Toggling this will allow only HTTPS secure endpoints,
													this allows only TLS connections to your endpoints.
												</FormDescription>
											</FormLabel>
										</div>
										<FormControl>
											<Switch
												disabled={
													!canManageProject || isUpdatingEnforceSecureEndpoints
												}
												checked={field.value}
												onCheckedChange={async e => {
													field.onChange(e);
													await updateEnforceSecureEndpoints(!field.value);
												}}
											/>
										</FormControl>
									</FormItem>
								)}
							/>
						</div>
					</div>
				</form>
			</Form>
		</section>
	);
}

const MetaEventsConfigFormSchema = z
	.object({
		is_enabled: z.boolean(),
		secret: z.string().transform(v => (v ? v : '')),
		url: z.string().transform(v => (v ? v : '')),
		event_type: z
			.array(z.enum(MetaEventTypes), {
				message: 'Event type(s) is required',
			})
			.nullable(),
	})
	.refine(
		config => {
			if (!config.is_enabled) return true;

			return !!config.url?.length && !!config.event_type?.length;
		},
		config => {
			if (!config.url?.length) {
				return {
					message: 'URL is required',
					path: ['url'],
				};
			}

			if (!config.event_type?.length) {
				return {
					message: 'At least one event type is required',
					path: ['event_type'],
				};
			}

			return { message: '' };
		},
	)
	.transform(config => {
		if (!config.is_enabled) {
			return {
				is_enabled: config.is_enabled,
				secret: '',
				url: '',
				event_type: null,
			};
		}

		return config;
	});

function MetaEventsConfig(props: {
	project: Project;
	canManageProject: boolean;
}) {
	const { project, canManageProject } = props;
	const [isUpdating, setIsUpdating] = useState(false);
	const { setProject, setProjects, projects } = useProjectStore();

	const [isMultiSelectOpen, setIsMultiSelectOpen] = useState(false);
	const [selectedEventTypes, setSelectedEventTypes] = useState<MetaEventType[]>(
		project.config.meta_event.event_type ?? [],
	);
	const [inputValue, setInputValue] = useState('');
	const handleUnselect = useCallback((eventType: MetaEventType) => {
		setSelectedEventTypes(prev => prev.filter(s => s !== eventType));
	}, []);
	const handleKeyDown = useCallback(
		(e: KeyboardEvent<HTMLInputElement>) => {
			if (e.key === 'Backspace' && selectedEventTypes.length > 0) {
				setSelectedEventTypes(prev => prev.slice(0, -1));
				return true;
			}
			return false;
		},
		[selectedEventTypes],
	);
	const filteredEventTypes = useMemo(
		() =>
			MetaEventTypes.filter(
				eventType => !selectedEventTypes.includes(eventType),
			),
		[selectedEventTypes],
	);

	const form = useForm<z.infer<typeof MetaEventsConfigFormSchema>>({
		resolver: zodResolver(MetaEventsConfigFormSchema),
		defaultValues: {
			is_enabled: project.config.meta_event.is_enabled,
			url: project.config.meta_event.url,
			secret: project.config.meta_event.secret,
			event_type:
				project.config.meta_event.event_type === null
					? []
					: project.config.meta_event.event_type,
		},
	});

	const isEventTypeEnabled = form.watch('is_enabled');

	async function updateMetaEventsConfig(
		metaEvent: z.infer<typeof MetaEventsConfigFormSchema>,
	) {
		setIsUpdating(true);
		try {
			project.config.meta_event = { ...metaEvent, type: '' };
			// @ts-expect-error this works
			const updated = await projectsService.updateProject(project);
			setProjects(projects.map(p => (p.uid == updated.uid ? updated : p)));
			setProject(updated);
		} catch (error) {
			console.error(error);
		} finally {
			setIsUpdating(false);
		}
	}

	return (
		<section className="flex flex-col gap-4">
			<h1 className="font-bold">Meta Event Configurations</h1>

			<Form {...form}>
				<form
					onSubmit={(...args) =>
						form.handleSubmit(updateMetaEventsConfig)(...args)
					}
				>
					<div className="space-y-4">
						<FormField
							control={form.control}
							name="is_enabled"
							render={({ field }) => (
								<FormItem className="flex flex-row items-center justify-between rounded-lg border p-3">
									<div className="space-y-0.5">
										<FormLabel className="font-semibold text-xs">
											Enable Meta Events
										</FormLabel>
										<FormDescription className="max-w-prose text-xs/5">
											Meta events allows you to receive webhook events based on
											events happening in your projects including webhook event
											activities.
										</FormDescription>
									</div>
									<FormControl>
										<Switch
											disabled={!canManageProject || isUpdating}
											checked={field.value}
											onCheckedChange={field.onChange}
										/>
									</FormControl>
								</FormItem>
							)}
						/>
					</div>
					<hr className="my-8" />

					{isEventTypeEnabled && (
						<div className="flex flex-col w-[75%]">
							<h2 className="text-xs font-semibold pb-2">
								Meta Events Configurations
							</h2>
							<FormField
								control={form.control}
								name="url"
								render={({ field, fieldState }) => (
									<FormItem className="w-full relative mb-6 block">
										<div className="w-full mb-2 flex items-center justify-between">
											<FormLabel className="text-xs/5 text-neutral-9">
												Webhook URL
											</FormLabel>
										</div>
										<FormControl>
											<Input
												autoComplete="on"
												type="url"
												className={cn(
													'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
													fieldState.error
														? 'border-destructive focus-visible:ring-0 hover:border-destructive'
														: ' hover:border-new.primary-100 focus:border-new.primary-300',
												)}
												{...field}
											/>
										</FormControl>
										<FormMessageWithErrorIcon />
									</FormItem>
								)}
							/>

							<FormField
								control={form.control}
								name="secret"
								render={({ field, fieldState }) => (
									<FormItem className="w-full relative mb-6 block">
										<div className="w-full mb-2 flex items-center justify-between">
											<FormLabel className="text-xs/5 text-neutral-9">
												Endpoint Secret
											</FormLabel>
										</div>
										<FormControl>
											<Input
												type="text"
												className={cn(
													'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
													fieldState.error
														? 'border-destructive focus-visible:ring-0 hover:border-destructive'
														: ' hover:border-new.primary-100 focus:border-new.primary-300',
												)}
												{...field}
											/>
										</FormControl>
										<FormMessageWithErrorIcon />
									</FormItem>
								)}
							/>

							<FormField
								control={form.control}
								name="event_type"
								render={({ field }) => (
									<FormItem className="w-full relative mb-6 block">
										<div className="w-full mb-2 flex items-center justify-between">
											<FormLabel className="text-xs/5 text-neutral-9">
												Select events to listen to
											</FormLabel>
										</div>
										<Command className="overflow-visible">
											<div className="rounded-md border border-input px-3 py-2 text-sm ring-offset-background focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2">
												<div className="flex flex-wrap gap-1">
													{selectedEventTypes.map(eventType => {
														return (
															<Badge
																key={eventType}
																variant="secondary"
																className="select-none"
															>
																{eventType}
																<X
																	className="size-3 text-muted-foreground hover:text-foreground ml-2 cursor-pointer"
																	onMouseDown={e => {
																		e.preventDefault();
																	}}
																	onClick={() => {
																		handleUnselect(eventType);
																		field.onChange(
																			selectedEventTypes.filter(
																				s => s !== eventType,
																			),
																		);
																	}}
																/>
															</Badge>
														);
													})}
													<CommandPrimitive.Input
														onKeyDown={e => {
															const isRemoveAction = handleKeyDown(e);
															if (isRemoveAction) {
																field.onChange(selectedEventTypes.slice(0, -1));
															}
														}}
														onValueChange={setInputValue}
														value={inputValue}
														onBlur={() => setIsMultiSelectOpen(false)}
														onFocus={() => setIsMultiSelectOpen(true)}
														placeholder=""
														className="ml-2 flex-1 bg-transparent outline-none placeholder:text-muted-foreground"
													/>
												</div>
											</div>
											<div className="relative mt-2">
												<CommandList>
													{isMultiSelectOpen && !!filteredEventTypes.length && (
														<div className="absolute top-0 z-10 w-full rounded-md border bg-popover text-popover-foreground shadow-md outline-none">
															<CommandGroup className="h-full overflow-auto">
																{filteredEventTypes.map(eventType => {
																	return (
																		<CommandItem
																			key={eventType}
																			onMouseDown={e => {
																				e.preventDefault();
																			}}
																			onSelect={() => {
																				setInputValue('');
																				setSelectedEventTypes(prev => {
																					field.onChange([...prev, eventType]);
																					return [...prev, eventType];
																				});
																			}}
																			className={'cursor-pointer'}
																		>
																			{eventType}
																		</CommandItem>
																	);
																})}
															</CommandGroup>
														</div>
													)}
												</CommandList>
											</div>
										</Command>
										<FormMessageWithErrorIcon />
									</FormItem>
								)}
							/>
						</div>
					)}

					<div className="flex justify-end">
						<Button
							disabled={
								!canManageProject || !form.formState.isValid || isUpdating
							}
							type="submit"
							variant="ghost"
							className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
						>
							Save Changes
						</Button>
					</div>
				</form>
			</Form>
		</section>
	);
}

const NewEventTypeFormSchema = z.object({
	name: z.string().min(1, 'Name is required'),
	category: z.string().optional(),
	description: z.string().optional(),
});

function EventTypesConfig(props: {
	project: Project;
	canManageProject: boolean;
	eventTypes: Array<EventType>;
}) {
	const { eventTypes, canManageProject, project } = props;
	const [isCreating, setIsCreating] = useState(false);
	const [_eventTypes, set_eventTypes] = useState(eventTypes);

	const form = useForm<z.infer<typeof NewEventTypeFormSchema>>({
		resolver: zodResolver(NewEventTypeFormSchema),
		defaultValues: {
			name: '',
			category: '',
			description: '',
		},
		mode: 'onTouched',
	});

	async function createNewEventType(
		eventType: z.infer<typeof NewEventTypeFormSchema>,
	) {
		setIsCreating(true);
		try {
			const created = await projectsService.createEventType(
				project.uid,
				eventType,
			);
			set_eventTypes(prev => prev.concat(created));
			// TODO should close the sheet here
		} catch (error) {
			console.error(error);
		} finally {
			setIsCreating(false);
		}
	}
	return (
		<section className="flex flex-col gap-4">
			<div className="flex items-center justify-between">
				<h1 className="font-bold">Event Types</h1>

				{/* This sheet is duplicated. Keep it single */}
				<Sheet>
					<SheetTrigger asChild>
						<Button
							disabled={!canManageProject}
							size={'sm'}
							variant="ghost"
							className="hover:bg-new.primary-400 px-3 text-xs hover:text-white-100 bg-new.primary-400 flex justify-between items-center"
						>
							<Plus className="stroke-white-100" />
							<p className=" text-white-100">Event Type</p>
						</Button>
					</SheetTrigger>
					<SheetContent className="w-[400px] px-0">
						<SheetHeader className="text-start px-4">
							<SheetTitle>New Event Type</SheetTitle>
							<SheetDescription className="sr-only">
								Add a new event type
							</SheetDescription>
						</SheetHeader>
						<hr className="my-6 border-neutral-5" />
						<div className="px-4">
							<Form {...form}>
								<form
									onSubmit={(...args) =>
										void form.handleSubmit(createNewEventType)(...args)
									}
								>
									<div>
										<FormField
											control={form.control}
											name="name"
											render={({ field, fieldState }) => (
												<FormItem className="w-full relative mb-6 block">
													<div className="w-full mb-2 flex items-center justify-between">
														<FormLabel className="text-xs/5 text-neutral-9">
															Name
														</FormLabel>
													</div>
													<FormControl>
														<Input
															type="text"
															className={cn(
																'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																fieldState.error
																	? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																	: ' hover:border-new.primary-100 focus:border-new.primary-300',
															)}
															{...field}
														/>
													</FormControl>
													<FormMessageWithErrorIcon />
												</FormItem>
											)}
										/>

										<FormField
											control={form.control}
											name="category"
											render={({ field, fieldState }) => (
												<FormItem className="w-full relative mb-6 block">
													<div className="w-full mb-2 flex items-center justify-between">
														<FormLabel className="text-xs/5 text-neutral-9">
															Category
														</FormLabel>
													</div>
													<FormControl>
														<Input
															autoComplete="on"
															type="text"
															className={cn(
																'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																fieldState.error
																	? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																	: ' hover:border-new.primary-100 focus:border-new.primary-300',
															)}
															{...field}
														/>
													</FormControl>
													<FormMessageWithErrorIcon />
												</FormItem>
											)}
										/>

										<FormField
											control={form.control}
											name="description"
											render={({ field, fieldState }) => (
												<FormItem className="w-full relative mb-6 block">
													<div className="w-full mb-2 flex items-center justify-between">
														<FormLabel className="text-xs/5 text-neutral-9">
															Description
														</FormLabel>
													</div>
													<FormControl>
														<Input
															autoComplete="on"
															type="text"
															className={cn(
																'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																fieldState.error
																	? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																	: ' hover:border-new.primary-100 focus:border-new.primary-300',
															)}
															{...field}
														/>
													</FormControl>
													<FormMessageWithErrorIcon />
												</FormItem>
											)}
										/>
									</div>
									<div className="flex justify-end items-center gap-x-4">
										<SheetClose asChild>
											<Button
												variant="ghost"
												className="hover:bg-white-100 text-destructive hover:text-destructive border border-destructive text-xs hover:destructive"
											>
												Discard
											</Button>
										</SheetClose>

										<Button
											type="submit"
											disabled={
												!canManageProject ||
												!form.formState.isValid ||
												isCreating
											}
											variant="ghost"
											className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
										>
											Create
										</Button>
									</div>
								</form>
							</Form>
						</div>
					</SheetContent>
				</Sheet>
			</div>

			<div>
				{/* TODO include empty state */}
				{_eventTypes.length == 0 ? (
					<div className="flex flex-col items-center">
						<img
							src={eventTypesEmptyState}
							alt="No event types created"
							className="mb-12 h-40"
						/>
						<p className="font-bold mb-4 text-base text-neutral-12 text-center">
							You currently do not have any event types
						</p>
						<p className="text-sm text-neutral-10 text-center mb-4">
							Event types will be listed here when available
						</p>
						{/* This sheet is duplicated. Keep it single */}
						<Sheet>
							<SheetTrigger asChild>
								<Button
									disabled={!canManageProject}
									size={'sm'}
									variant="ghost"
									className="hover:bg-new.primary-400 px-3 text-xs hover:text-white-100 bg-new.primary-400 flex justify-between items-center"
								>
									<Plus className="stroke-white-100" />
									<p className=" text-white-100">Create Event Type</p>
								</Button>
							</SheetTrigger>
							<SheetContent className="w-[400px] px-0">
								<SheetHeader className="text-start px-4">
									<SheetTitle>New Event Type</SheetTitle>
									<SheetDescription className="sr-only">
										Add a new event type
									</SheetDescription>
								</SheetHeader>
								<hr className="my-6 border-neutral-5" />
								<div className="px-4">
									<Form {...form}>
										<form
											onSubmit={(...args) =>
												void form.handleSubmit(createNewEventType)(...args)
											}
										>
											<div>
												<FormField
													control={form.control}
													name="name"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative mb-6 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Name
																</FormLabel>
															</div>
															<FormControl>
																<Input
																	type="text"
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="category"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative mb-6 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Category
																</FormLabel>
															</div>
															<FormControl>
																<Input
																	autoComplete="on"
																	type="text"
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="description"
													render={({ field, fieldState }) => (
														<FormItem className="w-full relative mb-6 block">
															<div className="w-full mb-2 flex items-center justify-between">
																<FormLabel className="text-xs/5 text-neutral-9">
																	Description
																</FormLabel>
															</div>
															<FormControl>
																<Input
																	autoComplete="on"
																	type="text"
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																	{...field}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>
											<div className="flex justify-end items-center gap-x-4">
												<SheetClose asChild>
													<Button
														variant="ghost"
														className="hover:bg-white-100 text-destructive hover:text-destructive border border-destructive text-xs hover:destructive"
													>
														Discard
													</Button>
												</SheetClose>

												<Button
													type="submit"
													disabled={
														!canManageProject ||
														!form.formState.isValid ||
														isCreating
													}
													variant="ghost"
													className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
												>
													Create
												</Button>
											</div>
										</form>
									</Form>
								</div>
							</SheetContent>
						</Sheet>
					</div>
				) : (
					<div>
						<Table>
							<TableCaption className="sr-only">
								{project.name} Event types
							</TableCaption>
							<TableHeader>
								<TableRow>
									<TableHead className=" uppercase text-new.black font-medium text-xs">
										event type
									</TableHead>
									<TableHead className="uppercase text-new.black font-medium text-xs">
										category
									</TableHead>
									<TableHead className="uppercase text-new.black font-medium text-xs">
										description
									</TableHead>
									<TableHead className="uppercase text-new.black font-medium text-xs"></TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{_eventTypes.map(et => {
									return (
										<TableRow key={et.uid}>
											<TableCell className="space-x-2">
												<span className="px-2 py-1 font-normal text-xs bg-neutral-a3 rounded-16px text-neutral-12">
													{et.name}
												</span>
												<span
													className={cn(
														'px-2 py-1 font-normal text-xs rounded-16px text-neutral-12',
														et.deprecated_at
															? 'bg-destructive/10 text-destructive'
															: 'bg-new.success-50 text-new.success-600',
													)}
												>
													{et.deprecated_at ? 'deprecated' : 'active'}
												</span>
											</TableCell>
											<TableCell className="text-xs font-normal text-neutral-12">
												{et.category || '-'}
											</TableCell>
											<TableCell className="text-xs font-normal text-neutral-12">
												{et.description || '-'}
											</TableCell>
											<TableCell className="flex items-center gap-x-2">
												<Button
													variant={'ghost'}
													className="p-1 bg-transparent hover:bg-transparent"
													disabled={!!et.deprecated_at}
												>
													<PencilLine className="stroke-neutral-10" />{' '}
													<span className="text-xs text-neutral-10">Edit</span>
												</Button>
												<Button
													variant={'ghost'}
													className="p-1 bg-transparent hover:bg-transparent"
													disabled={!!et.deprecated_at}
												>
													<Trash2 className="stroke-destructive" />{' '}
													<span className="text-xs text-destructive">
														Deprecate
													</span>
												</Button>
											</TableCell>
										</TableRow>
									);
								})}
							</TableBody>
						</Table>
					</div>
				)}
			</div>
		</section>
	);
}

const tabs = [
	{
		name: 'Project',
		value: 'projects',
		icon: Home,
		component: ProjectConfig,
		projectTypes: ['incoming', 'outgoing'],
	},
	{
		name: 'Signature History',
		value: 'signature-history',
		icon: Home,
		component: SignatureHistoryConfig,
		projectTypes: ['outgoing'],
	},
	{
		name: 'Endpoints',
		value: 'endpoints',
		icon: User,
		component: EndpointsConfig,
		projectTypes: ['incoming', 'outgoing'],
	},
	{
		name: 'Meta Events',
		value: 'meta-events',
		icon: Bot,
		component: MetaEventsConfig,
		projectTypes: ['incoming', 'outgoing'],
	},
	{
		name: 'Event Types',
		value: 'event-types',
		icon: Home,
		component: EventTypesConfig,
		projectTypes: ['outgoing'],
	},
	{
		name: 'Secrets',
		value: 'secrets',
		icon: Settings,
		component: ProjectConfig,
		projectTypes: ['incoming', 'outgoing'],
	},
];

function ProjectSettings() {
	const { project, canManageProject, eventTypes } = Route.useLoaderData();

	return (
		<DashboardLayout showSidebar={true}>
			<div className="">
				<section className="flex flex-col mx-4 ">
					<h1 className="">Project Settings</h1>
					<div>
						<Tabs
							orientation="vertical"
							defaultValue={tabs[0].value}
							className="w-full flex items-start gap-4 justify-center"
						>
							<TabsList className="shrink-0 grid grid-cols-1 min-w-[20%] p-0 gap-y-2 bg-background">
								{tabs
									.filter(tab => tab.projectTypes.includes(project.type))
									.map(tab => (
										<TabsTrigger
											key={tab.value}
											value={tab.value}
											className="border-l-2 border-transparent justify-start rounded-none data-[state=active]:shadow-none data-[state=active]:border-primary data-[state=active]:bg-primary/5 py-1.5"
										>
											<tab.icon className="h-5 w-5 me-2" /> {tab.name}
										</TabsTrigger>
									))}
							</TabsList>

							<div className="w-full">
								{tabs.map(tab => (
									<TabsContent key={tab.value} value={tab.value}>
										<tab.component
											project={project}
											canManageProject={canManageProject}
											eventTypes={eventTypes}
										/>
									</TabsContent>
								))}
							</div>
						</Tabs>
					</div>
				</section>
			</div>
		</DashboardLayout>
	);
}
