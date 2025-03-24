import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useParams, createFileRoute, Link } from '@tanstack/react-router';
import { useNavigate } from '@tanstack/react-router';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import {
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from '@/components/ui/tooltip';
import { DashboardLayout } from '@/components/dashboard';
import {
	Form,
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';

import * as authService from '@/services/auth.service';
import { endpointsService } from '@/services/endpoints.service';
import { useLicenseStore, useProjectStore } from '@/store/index';
import { ensureCanAccessPrivatePages } from '@/lib/auth';
import { cn } from '@/lib/utils';

import type { EndpointFormValues } from '@/models/endpoint.model';

// Import back button icon
import modalCloseIcon from '../../../../../assets/svg/modal-close-icon.svg';

// Schema for form validation
const endpointSchema = z.object({
	name: z.string().min(1, { message: 'Please provide a name' }),
	url: z.string().url({ message: 'Invalid endpoint URL' }),
	support_email: z
		.string()
		.email({ message: 'Email is invalid' })
		.optional()
		.or(z.literal('')),
	slack_webhook_url: z
		.string()
		.url({ message: 'URL is invalid' })
		.optional()
		.or(z.literal('')),
	secret: z.string().nullable(),
	http_timeout: z.number().nullable(),
	description: z.string().nullable(),
	owner_id: z.string().nullable(),
	rate_limit: z.number().nullable(),
	rate_limit_duration: z.number().nullable(),
	authentication: z
		.object({
			type: z.string(),
			api_key: z.object({
				header_name: z.string(),
				header_value: z.string(),
			}),
		})
		.optional(),
	advanced_signatures: z.boolean().nullable(),
});

type ConfigItem = {
	uid: string;
	name: string;
	show: boolean;
	deleted: boolean;
};

export const Route = createFileRoute('/projects_/$projectId/endpoints/new')({
	beforeLoad({ context }) {
		ensureCanAccessPrivatePages(context.auth?.getTokens().isLoggedIn);
	},
	loader: async () => {
		const perms = await authService.getUserPermissions();

		return {
			canManageEndpoints: perms.includes('Endpoints|MANAGE'),
		};
	},
	component: CreateEndpointPage,
});

function CreateEndpointPage() {
	// Router and navigation hooks
	const navigate = useNavigate();
	const { canManageEndpoints } = Route.useLoaderData();
	const params = useParams({ from: '/projects_/$projectId/endpoints/new' });

	// Global state from Zustand
	const { licenses } = useLicenseStore();
	const { project } = useProjectStore();

	// State
	const [configurations, setConfigurations] = useState<ConfigItem[]>(
		[
			{ uid: 'http_timeout', name: 'Timeout', show: false, deleted: false },
			{ uid: 'owner_id', name: 'Owner ID', show: false, deleted: false },
			{ uid: 'rate_limit', name: 'Rate Limit', show: false, deleted: false },
			{ uid: 'auth', name: 'Auth', show: false, deleted: false },
			{
				uid: 'alert_config',
				name: 'Notifications',
				show: false,
				deleted: false,
			},
		].concat(
			project?.type === 'outgoing'
				? [
						{
							uid: 'signature',
							name: 'Signature Format',
							show: false,
							deleted: false,
						},
					]
				: [],
		),
	);
	const [isCreating, setIsCreating] = useState(false);

	// Check if user has required license
	const hasAdvancedEndpointManagementLicense =
		Array.isArray(licenses) &&
		licenses.includes('ADVANCED_ENDPOINT_MANAGEMENT');

	// React Hook Form setup
	const form = useForm<EndpointFormValues>({
		resolver: zodResolver(endpointSchema),
		defaultValues: {
			name: '',
			url: '',
			support_email: '',
			slack_webhook_url: '',
			secret: null,
			http_timeout: null,
			description: null,
			owner_id: null,
			rate_limit: null,
			rate_limit_duration: null,
			authentication: {
				type: 'api_key',
				api_key: {
					header_name: '',
					header_value: '',
				},
			},
			advanced_signatures: null,
		},
		mode: 'onTouched'
	});

	const { register, setValue, watch, reset, control } = form;

	// Toggle configuration display
	const toggleConfigForm = (configValue: string, deleted?: boolean) => {
		setConfigurations(prev =>
			prev.map(config =>
				config.uid === configValue
					? { ...config, show: !config.show, deleted: deleted ?? false }
					: config,
			),
		);
	};

	// Set configuration as deleted
	const setConfigFormDeleted = (configValue: string, deleted: boolean) => {
		setConfigurations(prev =>
			prev.map(config =>
				config.uid === configValue ? { ...config, deleted } : config,
			),
		);
	};

	// Check if configuration is shown
	const showConfig = (configValue: string): boolean => {
		return (
			configurations.find(config => config.uid === configValue)?.show || false
		);
	};

	// Check if configuration is marked as deleted
	const configDeleted = (configValue: string): boolean => {
		return (
			configurations.find(config => config.uid === configValue)?.deleted ||
			false
		);
	};

	// Form submission handler
	const saveEndpoint = async (formData: EndpointFormValues) => {
		// Handle rate limit deleted case
		const rateLimitDeleted =
			!showConfig('rate_limit') && configDeleted('rate_limit');
		if (rateLimitDeleted) {
			formData.rate_limit = 0;
			formData.rate_limit_duration = 0;
			setConfigFormDeleted('rate_limit', false);
		}

		setIsCreating(true);

		// Clone form data to avoid mutating the original
		// Fix type conversion by first going through unknown
		const endpointValue = structuredClone(formData) as unknown as Record<
			string,
			unknown
		>;

		// Remove authentication if both fields are empty
		if (
			formData.authentication &&
			!formData.authentication.api_key.header_name &&
			!formData.authentication.api_key.header_value
		) {
			delete endpointValue.authentication;
		}

		try {
			// Create new endpoint
			const response = await endpointsService.addEndpoint(endpointValue);
			// toast.success(response.message || 'Endpoint created successfully');

			reset();

			// Navigate back to endpoints list
			navigate({ to: `/projects/${params.projectId}/endpoints` });
		} catch (error) {
			// toast.error('Failed to save endpoint');
			console.error(error);
		} finally {
			setIsCreating(false);
		}
	};

	return (
		<DashboardLayout showSidebar={false}>
			<section className="flex flex-col p-2 max-w-[770px] min-w-[600px] w-full m-auto my-4">
				<div className="flex justify-start items-center gap-2">
					<Link
						to="/projects/$projectId/endpoints"
						params={{ projectId: params.projectId }}
						className="flex justify-center items-center p-2 bg-new.primary-25 rounded-8px"
						activeProps={{}}
					>
						<img
							src={modalCloseIcon}
							alt="back to endpoints"
							className="h-3 w-3"
						/>
					</Link>
					<h1 className="font-semibold text-sm">Create Endpoint</h1>
				</div>

				<p className="text-xs/5 text-neutral-11 my-3">
					An endpoint represents a destination for your webhook events.
					Configure your endpoint details below.
				</p>

				<div className="border border-new.primary-50 rounded-8px p-6 relative w-full">
					<Form {...form}>
						<form onSubmit={form.handleSubmit(saveEndpoint)} className="w-full">
							<div className="grid gap-6 grid-cols-1 md:grid-cols-2 w-full">
								<FormField
									control={control}
									name="name"
									render={({ field, fieldState }) => (
										<FormItem className="w-full relative block">
											<FormLabel className="required text-xs/5 text-neutral-9">
												Endpoint Name
											</FormLabel>
											<FormControl>
												<Input
													{...field}
													placeholder="Enter endpoint name here"
													className={cn(
														'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
														fieldState.error
															? 'border-destructive focus-visible:ring-0 hover:border-destructive'
															: 'hover:border-new.primary-100 focus:border-new.primary-300',
													)}
												/>
											</FormControl>
											<FormMessageWithErrorIcon />
										</FormItem>
									)}
								/>

								<FormField
									control={control}
									name="url"
									render={({ field, fieldState }) => (
										<FormItem className="w-full relative block">
											<FormLabel className="required text-xs/5 text-neutral-9">
												Enter URL
											</FormLabel>
											<FormControl>
												<Input
													type="url"
													{...field}
													placeholder="Enter endpoint URL here"
													className={cn(
														'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
														fieldState.error
															? 'border-destructive focus-visible:ring-0 hover:border-destructive'
															: 'hover:border-new.primary-100 focus:border-new.primary-300',
													)}
												/>
											</FormControl>
											<FormMessageWithErrorIcon />
										</FormItem>
									)}
								/>
							</div>

							<div className="space-y-2 mt-6 mb-6 w-full">
								<Label htmlFor="secret" className="text-xs/5 text-neutral-9">
									Endpoint Secret
								</Label>
								<Input
									id="secret"
									type="text"
									{...register('secret')}
									placeholder="Enter endpoint secret here"
									className="mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25 hover:border-new.primary-100 focus:border-new.primary-300"
								/>
							</div>

							{/* Owner ID Configuration */}
							{showConfig('owner_id') && (
								<div className="border-l-2 border-primary-100 pl-4 mb-10 w-full">
									<div className="flex justify-between items-center mb-4">
										<div className="flex items-center">
											<Label
												htmlFor="owner_id"
												className="text-xs/5 text-neutral-9"
											>
												Owner ID
											</Label>
											<TooltipProvider>
												<Tooltip>
													<TooltipTrigger asChild>
														<span className="ml-1 cursor-help">ⓘ</span>
													</TooltipTrigger>
													<TooltipContent>
														A unique id for identifying a group of endpoints,
														like a user id. Useful for fanning out an event to
														multiple endpoints and creating portal link for
														multiple endpoints.
													</TooltipContent>
												</Tooltip>
											</TooltipProvider>
										</div>
										<Button
											type="button"
											variant="outline"
											size="sm"
											onClick={() => toggleConfigForm('owner_id', true)}
										>
											Remove
										</Button>
									</div>

									<div className="space-y-2">
										<Input
											id="owner_id"
											{...register('owner_id')}
											placeholder="Enter owner id here"
											className="mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25 hover:border-new.primary-100 focus:border-new.primary-300"
										/>
									</div>
								</div>
							)}

							{/* Configuration wrapper to maintain width consistency */}
							<div className="w-full">
								{/* Rate Limit Configuration */}
								{showConfig('rate_limit') && (
									<div className="border-l-2 border-primary-100 pl-4 mb-10 w-full">
										<div className="flex justify-between items-center mb-4">
											<p className="text-sm text-gray-700 font-medium">
												Rate Limit
											</p>
											<Button
												type="button"
												variant="outline"
												size="sm"
												onClick={() => toggleConfigForm('rate_limit', true)}
											>
												Remove
											</Button>
										</div>

										<div className="grid grid-cols-2 gap-6 w-full">
											<FormField
												control={control}
												name="rate_limit_duration"
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel
															htmlFor="rate-limit-duration"
															className="text-xs/5 text-neutral-9"
														>
															Duration
														</FormLabel>
														<FormControl>
															<div className="relative">
																<Input
																	id="rate-limit-duration"
																	type="number"
																	inputMode="numeric"
																	pattern="\d*"
																	min={0}
																	value={field.value?.toString() || ''}
																	onChange={e => {
																		const value =
																			e.target.value === ''
																				? null
																				: Number(e.target.value);
																		field.onChange(value);
																	}}
																	onBlur={field.onBlur}
																	name={field.name}
																	ref={field.ref}
																	placeholder="e.g 50"
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: 'hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																/>
																<div className="absolute top-1/2 right-3 text-base text-gray-400 opacity-40 -translate-y-1/2">
																	sec
																</div>
															</div>
														</FormControl>
														<FormMessageWithErrorIcon />
													</FormItem>
												)}
											/>

											<FormField
												control={control}
												name="rate_limit"
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel
															htmlFor="rate-limit-count"
															className="text-xs/5 text-neutral-9"
														>
															Limit
														</FormLabel>
														<FormControl>
															<Input
																id="rate-limit-count"
																type="number"
																inputMode="numeric"
																pattern="\d*"
																min={1}
																value={field.value?.toString() || ''}
																onChange={e => {
																	const value =
																		e.target.value === ''
																			? null
																			: Number(e.target.value);
																	field.onChange(value);
																}}
																onBlur={field.onBlur}
																name={field.name}
																ref={field.ref}
																placeholder="e.g 10"
																className={cn(
																	'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																	fieldState.error
																		? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																		: 'hover:border-new.primary-100 focus:border-new.primary-300',
																)}
															/>
														</FormControl>
														<FormMessageWithErrorIcon />
													</FormItem>
												)}
											/>
										</div>
									</div>
								)}

								{/* Alert Configuration */}
								{showConfig('alert_config') && (
									<div className="border-l-2 border-primary-100 pl-4 mb-10 w-full">
										<div className="flex justify-between items-center mb-4">
											<div className="flex items-center gap-3">
												<p className="text-sm text-gray-700 font-medium">
													Alert Configuration
												</p>
												{!hasAdvancedEndpointManagementLicense && (
													<span className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium bg-new.primary-25 text-new.primary-700">
														<svg
															width="10"
															height="10"
															className="fill-new.primary-400 scale-150"
														>
															<use xlinkHref="#lock-icon"></use>
														</svg>
														Business
													</span>
												)}
											</div>
											<Button
												type="button"
												variant="outline"
												size="sm"
												onClick={() => toggleConfigForm('alert_config', true)}
											>
												Remove
											</Button>
										</div>

										{hasAdvancedEndpointManagementLicense && (
											<div className="grid grid-cols-2 gap-6 w-full">
												<FormField
													control={control}
													name="support_email"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel
																htmlFor="support-email"
																className="text-xs/5 text-neutral-9"
															>
																Support Email
															</FormLabel>
															<FormControl>
																<Input
																	id="support-email"
																	{...field}
																	placeholder="Enter support email"
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: 'hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													control={control}
													name="slack_webhook_url"
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel
																htmlFor="slack-url"
																className="text-xs/5 text-neutral-9"
															>
																Slack webhook url
															</FormLabel>
															<FormControl>
																<Input
																	id="slack-url"
																	{...field}
																	placeholder="Enter slack webhook URL"
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: 'hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>
										)}
									</div>
								)}

								{/* Authentication Configuration */}
								{showConfig('auth') && (
									<div className="border-l-2 border-primary-100 pl-4 mb-10 w-full">
										<div className="flex justify-between items-center mb-4">
											<div className="flex items-center">
												<p className="text-sm font-medium text-gray-700">
													Endpoint Authentication
												</p>
												<TooltipProvider>
													<Tooltip>
														<TooltipTrigger asChild>
															<span className="ml-1 cursor-help">ⓘ</span>
														</TooltipTrigger>
														<TooltipContent>
															You can set your provided endpoint authentication
															if any is required
														</TooltipContent>
													</Tooltip>
												</TooltipProvider>
											</div>
											<Button
												type="button"
												variant="outline"
												size="sm"
												onClick={() => toggleConfigForm('auth', true)}
											>
												Remove
											</Button>
										</div>

										<div className="grid grid-cols-2 gap-6 w-full">
											<FormField
												control={control}
												name="authentication.api_key.header_name"
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel
															htmlFor="header_name"
															className="text-xs/5 text-neutral-9"
														>
															API Key Name
														</FormLabel>
														<FormControl>
															<Input
																id="header_name"
																{...field}
																placeholder="Name"
																className={cn(
																	'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																	fieldState.error
																		? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																		: 'hover:border-new.primary-100 focus:border-new.primary-300',
																)}
															/>
														</FormControl>
														<FormMessageWithErrorIcon />
													</FormItem>
												)}
											/>

											<FormField
												control={control}
												name="authentication.api_key.header_value"
												render={({ field, fieldState }) => (
													<FormItem className="space-y-2">
														<FormLabel
															htmlFor="header_value"
															className="text-xs/5 text-neutral-9"
														>
															API Key Value
														</FormLabel>
														<FormControl>
															<Input
																id="header_value"
																{...field}
																placeholder="Value"
																className={cn(
																	'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																	fieldState.error
																		? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																		: 'hover:border-new.primary-100 focus:border-new.primary-300',
																)}
															/>
														</FormControl>
														<FormMessageWithErrorIcon />
													</FormItem>
												)}
											/>
										</div>
									</div>
								)}

								{/* Signature Format Configuration */}
								{showConfig('signature') && (
									<div className="border-l-2 border-primary-100 pl-4 mb-10 w-full">
										<div className="flex justify-between items-center mb-4">
											<div className="flex items-center">
												<p className="text-sm font-medium text-gray-700">
													Signature Format
												</p>
												<TooltipProvider>
													<Tooltip>
														<TooltipTrigger asChild>
															<span className="ml-1 cursor-help">ⓘ</span>
														</TooltipTrigger>
														<TooltipContent>
															This specifies your signature format for your
															project.
														</TooltipContent>
													</Tooltip>
												</TooltipProvider>
											</div>
											<Button
												type="button"
												variant="outline"
												size="sm"
												onClick={() => toggleConfigForm('signature', true)}
											>
												Remove
											</Button>
										</div>

										<RadioGroup
											defaultValue={
												watch('advanced_signatures') ? 'false' : 'true'
											}
											className="grid grid-cols-2 gap-6 mb-10 w-full"
											onValueChange={value =>
												setValue('advanced_signatures', value === 'true')
											}
										>
											<div className="flex items-start">
												<label
													className={cn(
														'cursor-pointer border w-full transition-all ease-in duration-200 flex items-start gap-x-2 p-4 rounded-sm',
														watch('advanced_signatures') === false
															? 'border-new.primary-300 bg-[#FAFAFE]'
															: 'border-neutral-5',
													)}
												>
													<RadioGroupItem
														value="false"
														id="simple"
														className="mt-0.5"
													/>
													<div className="flex flex-col gap-y-1">
														<h4 className="w-full text-xs text-neutral-10 font-semibold">
															Simple
														</h4>
													</div>
												</label>
											</div>
											<div className="flex items-start">
												<label
													className={cn(
														'cursor-pointer border w-full transition-all ease-in duration-200 flex items-start gap-x-2 p-4 rounded-sm',
														watch('advanced_signatures') === true
															? 'border-new.primary-300 bg-[#FAFAFE]'
															: 'border-neutral-5',
													)}
												>
													<RadioGroupItem
														value="true"
														id="advanced"
														className="mt-0.5"
													/>
													<div className="flex flex-col gap-y-1">
														<h4 className="w-full text-xs text-neutral-10 font-semibold">
															Advanced
														</h4>
													</div>
												</label>
											</div>
										</RadioGroup>
									</div>
								)}

								{/* HTTP Timeout Configuration */}
								{showConfig('http_timeout') && (
									<div className="border-l-2 border-primary-100 pl-4 mb-10 w-full">
										<div className="flex justify-between items-center mb-4">
											<div className="flex items-center gap-2">
												<div className="flex items-center">
													<p className="text-sm font-medium text-gray-700">
														Endpoint Timeout
													</p>
													<TooltipProvider>
														<Tooltip>
															<TooltipTrigger asChild>
																<span className="ml-1 cursor-help">ⓘ</span>
															</TooltipTrigger>
															<TooltipContent>
																How many seconds should Convoy wait for a
																response from this endpoint before timing out?
															</TooltipContent>
														</Tooltip>
													</TooltipProvider>
												</div>
												{!hasAdvancedEndpointManagementLicense && (
													<span className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium bg-new.primary-25 text-new.primary-700">
														<svg
															width="10"
															height="10"
															className="fill-new.primary-400 scale-150"
														>
															<use xlinkHref="#lock-icon"></use>
														</svg>
														Business
													</span>
												)}
											</div>
											<Button
												type="button"
												variant="outline"
												size="sm"
												onClick={() => toggleConfigForm('http_timeout', true)}
											>
												Remove
											</Button>
										</div>

										<FormField
											control={control}
											name="http_timeout"
											render={({ field, fieldState }) => (
												<FormItem className="space-y-2">
													<FormLabel
														htmlFor="http_timeout"
														className="text-xs/5 text-neutral-9"
													>
														Timeout Value
													</FormLabel>
													<FormControl>
														<div className="relative">
															<Input
																id="http_timeout"
																type="number"
																inputMode="numeric"
																pattern="\d*"
																min={0}
																step={1}
																value={field.value?.toString() || ''}
																onChange={e => {
																	const value =
																		e.target.value === ''
																			? null
																			: Number(e.target.value);
																	field.onChange(value);
																}}
																onBlur={field.onBlur}
																name={field.name}
																ref={field.ref}
																placeholder="e.g 60"
																readOnly={!hasAdvancedEndpointManagementLicense}
																className={cn(
																	'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																	fieldState.error
																		? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																		: 'hover:border-new.primary-100 focus:border-new.primary-300',
																)}
															/>
															<div className="absolute top-1/2 right-3 text-base text-gray-400 opacity-40 -translate-y-1/2">
																sec
															</div>
														</div>
													</FormControl>
													<FormMessageWithErrorIcon />
												</FormItem>
											)}
										/>
									</div>
								)}
							</div>

							{/* Configuration Buttons */}
							<div className="flex items-center gap-3 overflow-x-auto no-scrollbar w-full">
								{configurations.map(
									config =>
										!config.show && (
											<Button
												key={config.uid}
												type="button"
												variant="outline"
												size="sm"
												onClick={() => toggleConfigForm(config.uid)}
												className="px-3 py-2 h-8 rounded-md text-xs font-medium bg-white border border-neutral-4 text-neutral-11 hover:border-new.primary-100 hover:text-new.primary-400 whitespace-nowrap"
											>
												{config.name}
											</Button>
										),
								)}
							</div>

							{/* Submit Button */}
							<div className="flex justify-end mt-10 mb-4 w-full">
								<Button
									type="submit"
									disabled={isCreating || !canManageEndpoints}
									variant="ghost"
									className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
								>
									{isCreating ? 'Creating...' : 'Create'} Endpoint
								</Button>
							</div>
						</form>
					</Form>

					{isCreating && (
						<div className="absolute inset-0 bg-white/80 flex items-center justify-center">
							<div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
						</div>
					)}
				</div>
			</section>
		</DashboardLayout>
	);
}
