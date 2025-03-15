import { z } from 'zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createFileRoute, useNavigate } from '@tanstack/react-router';

import { Bot, Home, Settings, User } from 'lucide-react';

import {
	Form,
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
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
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { DashboardLayout } from '@/components/dashboard';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';

import { cn } from '@/lib/utils';
import { useProjectStore } from '@/store/index';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as authService from '@/services/auth.service';
import * as projectsService from '@/services/projects.service';

import type { Project } from '@/models/project.model';

import warningAnimation from '../../../../assets/img/warning-animation.gif';

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
					search_policy: _project.config.search_policy,
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
					encoding: _project.config.signature.versions.at(0)?.encoding,
					// @ts-expect-error a default value exists, even for incoming projects
					hash: _project.config.signature.versions.at(0)?.hash,
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
					search_policy: (payload.config?.search_policy?.isEnabled
						? payload.config.search_policy.search_policy
						: _project.config.search_policy) as `${string}h`,
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
		return { project, canManageProject };
	},
	component: ProjectSettings,
});

const tabs = [
	{
		name: 'Projects',
		value: 'projects',
		icon: Home,
		component: ProjectConfig,
	},
	{
		name: 'Endpoints',
		value: 'endpoints',
		icon: User,
		component: ProjectConfig,
	},
	{
		name: 'Meta Events',
		value: 'meta-events',
		icon: Bot,
		component: ProjectConfig,
	},
	{
		name: 'Secrets',
		value: 'secrets',
		icon: Settings,
		component: ProjectConfig,
	},
];

function ProjectSettings() {
	const { project, canManageProject } = Route.useLoaderData();

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
								{tabs.map(tab => (
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
