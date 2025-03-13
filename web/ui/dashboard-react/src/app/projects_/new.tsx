import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { useEffect, useState } from 'react';
import { zodResolver } from '@hookform/resolvers/zod';
import { createFileRoute, Link } from '@tanstack/react-router';

import { CopyIcon } from 'lucide-react';

import {
	Dialog,
	DialogTrigger,
	DialogContent,
	DialogTitle,
	DialogClose,
	DialogHeader,
	DialogFooter,
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
import { Form } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import { DashboardLayout } from '@/components/dashboard';

import { cn } from '@/lib/utils';
import * as authService from '@/services/auth.service';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import * as projectsService from '@/services/projects.service';

import modalCloseIcon from '../../../assets/svg/modal-close-icon.svg';
import successAnimation from '../../../assets/img/success.gif';

export const Route = createFileRoute('/projects_/new')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	async loader() {
		const userPerms = await authService.getUserPermissions();

		return {
			canCreateProject: userPerms.includes('Project Settings|MANAGE'),
		};
	},
	component: CreateNewProject,
});

const CreateProjectFormSchema = z.object({
	name: z
		.string({
			required_error: 'Project name is required',
		})
		.min(1, 'Project name is required'),
	type: z
		.enum(['incoming', 'outgoing', ''], {
			required_error: 'Please select a project type',
		})
		.refine(v => v.length != 0, {
			message: 'Choose a project webhook type',
		}),
	config: z
		.object({
			search_policy: z
				.object({
					isEnabled: z.boolean().optional(),
					search_policy: z
						.string()
						.optional()
						.transform(v => (!v?.length ? undefined : `${v}h`)),
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

function CreateNewProject() {
	const [hasCreatedProject, setHasCreatedProject] = useState(false);
	const [projectkey, setProjectkey] = useState('');
	const { canCreateProject } = Route.useLoaderData();
	const form = useForm<z.infer<typeof CreateProjectFormSchema>>({
		resolver: zodResolver(CreateProjectFormSchema),
		defaultValues: {
			name: '',
			type: '',
			config: {
				search_policy: {
					isEnabled: false,
					search_policy: '',
				},
				ratelimit: {
					isEnabled: false,
					// @ts-expect-error the input values are strings, so this is correct. There is transform that converts this to a number
					count: '',
					// @ts-expect-error the input values are strings, so this is correct. There is transform that converts this to a number
					duration: '',
				},
				strategy: {
					isEnabled: false,
					// @ts-expect-error the input values are strings, so this is correct. There is transform that converts this to a number
					duration: '',
					// @ts-expect-error the input values are strings, so this is correct. There is transform that converts this to a number
					retry_count: '',
					type: 'linear',
				},
				signature: {
					isEnabled: false,
					header: '',
					encoding: '',
					hash: '',
				},
			},
		},
		mode: 'onTouched',
	});

	const selectedWebhookType = form.watch('type');
	const shouldShowRetryConfig = form.watch('config.strategy.isEnabled');
	const shouldShowRateLimit = form.watch('config.ratelimit.isEnabled');
	const shouldShowSearchPolicy = form.watch('config.search_policy.isEnabled');
	const shouldShowSigFormat = form.watch('config.signature.isEnabled');

	useEffect(() => {
		if (selectedWebhookType != 'outgoing') {
			form.setValue('config.signature.isEnabled', false);
		}
	}, [selectedWebhookType]);

	async function createProject(
		values: z.infer<typeof CreateProjectFormSchema>,
	) {
		let payload = {
			name: values.name,
			type: values.type as Exclude<
				z.infer<typeof CreateProjectFormSchema>['type'],
				''
			>,
			config: {} as z.infer<typeof CreateProjectFormSchema>['config'],
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
			const { api_key } = await projectsService.createProject({
				name: payload.name,
				type: payload.type,
				// @ts-expect-error it works. source: track/debug the code
				config: {
					...payload.config,
					...(payload.config?.search_policy?.search_policy && {
						search_policy: payload.config?.search_policy
							?.search_policy as `${string}h`,
					}),
					...(payload.config?.signature && {
						signature: {
							header: payload.config.signature.header,
							versions: [
								{
									hash: payload.config.signature.hash,
									encoding: payload.config.signature.encoding,
								},
							],
						},
					}),
				},
			});
			setProjectkey(api_key.key);
			setHasCreatedProject(true);
			form.reset();
		} catch (error) {
			// TODO: notify UI of error
			console.error(error);
		}
	}

	const webhookTypeOptions = [
		{
			type: 'incoming',
			desc: 'Create an incoming webhooks project to proxy events from third-party providers to your endpoints.',
		},
		{
			type: 'outgoing',
			desc: 'Create an outgoing webhooks project to publish events to your customer-provided endpoints.',
		},
	];

	return (
		<DashboardLayout showSidebar={false}>
			<section className="flex flex-col p-2 max-w-[770px] m-auto my-4">
				<div className="flex justify-start items-center gap-2">
					<Link
						to="/projects"
						className="flex justify-center items-center p-2 bg-new.primary-25 rounded-8px"
						activeProps={{}}
					>
						<img
							src={modalCloseIcon}
							alt="go to projects page"
							className="h-3 w-3 "
						/>
					</Link>
					<h1 className="font-semibold text-sm">Create Project</h1>
				</div>

				<p className="text-xs/5 text-neutral-11 my-3">
					A project represents the top level namespace for grouping event
					sources, applications, endpoints and events.
				</p>

				<Form {...form}>
					<form
						onSubmit={(...args) =>
							void form.handleSubmit(createProject)(...args)
						}
					>
						<div className="p-6 mb-6 border border-new.primary-50 rounded-8px">
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

							<FormField
								control={form.control}
								name="type"
								render={({ field }) => (
									<FormItem className="w-full relative mb-6 block">
										<p className="text-xs/5 text-neutral-9 mb-2">
											Project type
										</p>
										<div className="flex w-full gap-x-6">
											{webhookTypeOptions.map(({ type, desc }) => {
												return (
													<FormControl className="w-full " key={type}>
														<label
															className={cn(
																'cursor-pointer border border-primary-100 transition-all ease-in duration-200 flex items-start gap-x-2 p-4 rounded-sm',
																field.value == type
																	? 'border-new.primary-300 bg-[#FAFAFE]'
																	: 'border-neutral-5',
															)}
														>
															<Input
																type="radio"
																{...field}
																value={type}
																className="shadow-none h-4 w-fit"
															/>
															<div className="flex flex-col gap-y-1">
																<h4 className="w-full text-xs text-neutral-10 font-semibold capitalize">
																	{type} webhooks
																</h4>
																<p className="text-neutral-11 text-xs/5 font-normal">
																	{desc}
																</p>
															</div>
														</label>
													</FormControl>
												);
											})}
										</div>
										<FormMessageWithErrorIcon />
									</FormItem>
								)}
							/>

							<div className="flex justify-between gap-4 my-2 w-[90%]">
								<label className="flex items-center gap-2 cursor-pointer">
									{/* TODO you may want to make this into a component */}
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
															disabled={selectedWebhookType !== 'outgoing'}
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
											selectedWebhookType != 'outgoing' ? 'opacity-50' : '',
										)}
									>
										Signature Format
									</span>
								</label>
							</div>

							<Accordion
								type="multiple"
								className="w-full transition-all duration-300 "
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
															<Select onValueChange={field.onChange}>
																<FormControl>
																	<SelectTrigger>
																		<SelectValue />
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

								{selectedWebhookType == 'outgoing' && shouldShowSigFormat ? (
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
															<Select onValueChange={field.onChange}>
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
															<Select onValueChange={field.onChange}>
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
								disabled={!canCreateProject || !form.formState.isValid}
								variant="ghost"
								className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
							>
								Create Project
							</Button>
						</div>
					</form>
				</Form>
			</section>
			<Dialog open={hasCreatedProject}>
				<DialogTrigger></DialogTrigger>
				<DialogContent
					className="sm:max-w-[432px] rounded-lg"
					aria-describedby={undefined}
				>
					<DialogHeader>
						<DialogTitle className="flex flex-col justify-center items-center">
							<img src={successAnimation} alt="warning" className="w-28" />
							<span className="text-sm font-semibold">
								Project Created Successfully
							</span>
						</DialogTitle>
						<div className="flex flex-col items-center gap-y-3">
							<div className="flex flex-col justify-center items-center font-normal text-neutral-11 text-xs/5">
								<span>Your API Key has also been created.</span>
								<span>Please copy this key and save it somewhere safe.</span>
							</div>

							<div className="flex items-center justify-between w-[400px] h-[50px] border border-neutral-a3 bg-[#F7F9FC] pr-2 pl-3 rounded-md">
								<span className="text-xs text-neutral-11 font-normal truncate">
									{projectkey}
								</span>
								<Button
									type="button"
									variant="ghost"
									size="sm"
									className="asbolute right-[1%] top-0 h-full py-2 hover:bg-transparent pr-1 pl-0"
									onClick={() => {
										window.navigator.clipboard.writeText(projectkey).then();
										// TODO show toast message on copy successful
									}}
								>
									<CopyIcon className="opacity-50" aria-hidden="true" />
									<span className="sr-only">copy project key</span>
								</Button>
							</div>
						</div>
					</DialogHeader>
					<DialogFooter className="flex justify-center items-center">
						<DialogClose asChild>
							<Button
								onClick={() => {
									setHasCreatedProject(false);
								}}
								type="button"
								variant="ghost"
								className="hover:bg-new.primary-400 text-white-100 hover:text-white-100 bg-new.primary-400 px-3 py-4 text-xs"
							>
								Done
							</Button>
						</DialogClose>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</DashboardLayout>
	);
}
