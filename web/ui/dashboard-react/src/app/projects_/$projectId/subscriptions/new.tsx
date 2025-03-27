import { z } from 'zod';
import { useEffect, useState } from 'react';
import { Editor } from '@monaco-editor/react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createFileRoute, useNavigate, Link } from '@tanstack/react-router';

import { Check, ChevronDown, Info, Save } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { InputTags } from '@/components/ui/input-tags';
import {
	Command,
	CommandItem,
	CommandList,
	CommandEmpty,
	CommandGroup,
	CommandInput,
} from '@/components/ui/command';
import {
	Form,
	FormControl,
	FormField,
	FormItem,
	FormLabel,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from '@/components/ui/popover';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from '@/components/ui/tooltip';
import { DashboardLayout } from '@/components/dashboard';
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from '@/components/ui/dialog';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

import { cn } from '@/lib/utils';
import { useLicenseStore, useProjectStore } from '@/store';
import { stringToJson } from '@/lib/pipes';
import * as subscriptionsService from '@/services/subscriptions.service';
import * as authService from '@/services/auth.service';
import * as endpointsService from '@/services/endpoints.service';
import * as projectsService from '@/services/projects.service';
import * as licenseService from '@/services/licenses.service';
import * as sourcesService from '@/services/sources.service';

import type { ENDPOINT } from '@/models/endpoint.model';
import type { SOURCE } from '@/models/source';

import githubIcon from '../../../../../assets/img/github.png';
import shopifyIcon from '../../../../../assets/img/shopify.png';
import twitterIcon from '../../../../../assets/img/twitter.png';
import modalCloseIcon from '../../../../../assets/svg/modal-close-icon.svg';

const CreateSourceFormSchema = z.object({
	name: z
		.string({ required_error: 'Enter new source name' })
		.min(1, 'Enter new source name'),
	type: z.enum([
		'noop',
		'hmac',
		'basic_auth',
		'api_key',
		'github',
		'shopify',
		'twitter',
	]),
	is_disabled: z.boolean().optional().default(false),
	source_id: z.string().optional(),
	config: z.object({
		// TODO: In refine, encoding, hash, header and secret are required if type == hmac
		encoding: z.enum(['base64', 'hex', '']).optional(),
		hash: z.enum(['SHA256', 'SHA512', '']).optional(),
		header: z.string().optional(),
		secret: z.string().optional(),
		username: z.string().optional(),
		password: z.string().optional(),
		header_name: z.string().optional(),
		header_value: z.string().optional(),
	}), // z.record(z.union([z.string(), z.boolean()])).optional(),
	custom_response: z
		.object({
			content_type: z.string().optional(),
			body: z.string().optional(),
		})
		.optional(),
	idempotency_keys: z.array(z.string()).optional(),
});

const CreateEndpointFormSchema = z.object({
	name: z.string().min(1, 'Please provide a name'),
	url: z.string().url('Invalid endpoint URL'),
	secret: z.string().optional(),
	showHttpTimeout: z.boolean(),
	showRateLimit: z.boolean(),
	showOwnerId: z.boolean(),
	showAuth: z.boolean(),
	showNotifications: z.boolean(),
	showSignatureFormat: z.boolean(),
	owner_id: z.string().optional(),
	http_timeout: z.string().optional(),
	rate_limit: z.string().optional(),
	rate_limit_duration: z.string().optional(),
	support_email: z.string().email('Email is invalid').optional(),
	slack_webhook_url: z.string().url('URL is invalid').optional(),
	authentication: z
		.object({
			type: z.string().default('api_key'),
			api_key: z
				.object({
					header_name: z.string().optional(),
					header_value: z.string().optional(),
				})
				.optional(),
		})
		.optional(),
	advanced_signatures: z.enum(['true', 'false']),
});

const CreateSubscriptionFormSchema = z.object({
	name: z.string().min(1, 'Enter new subscription name'),
	function: z.string().optional(), // transform function
	filter_config: z.object({
		event_types: z.array(z.string()),
		filter: z
			.object({
				headers: z.union([z.record(z.string(), z.any()), z.object({})]),
				body: z.union([z.record(z.string(), z.any()), z.object({})]),
			})
			.optional(),
	}),
	showEventsFilter: z.boolean(),
	showEventTypes: z.boolean(),
	showTransform: z.boolean(),
	source_id: z.string({ required_error: 'Select source' }),
	source: CreateSourceFormSchema.optional(),
	endpoint_id: z.string().optional(),
	endpoint: CreateEndpointFormSchema.optional(),
});

export const Route = createFileRoute('/projects_/$projectId/subscriptions/new')(
	{
		component: CreateSubscriptionPage,
		loader: async function () {
			const perms = await authService.getUserPermissions();
			const sources = await sourcesService.getSources({});
			const endpoints = await endpointsService.getEndpoints();
			const licenses = useLicenseStore.getState().licenses;
			const hasAdvancedEndpointManagement = licenses.includes(
				'ADVANCED_ENDPOINT_MANAGEMENT',
			);
			const hasAdvancedSubscriptions = licenses.includes(
				'ADVANCED_SUBSCRIPTIONS',
			);
			const hasWebhookTransformations = licenses.includes(
				'WEBHOOK_TRANSFORMATIONS',
			);
			return {
				canManageSubscriptions: perms.includes('Subscriptions|MANAGE'),
				existingSources: sources.content,
				existingEndpoints: endpoints.data.content,
				hasAdvancedEndpointManagement,
				hasAdvancedSubscriptions,
				hasWebhookTransformations,
			};
		},
	},
);

const monacoEditorOptions = {
	formatOnPaste: true,
	formatOnType: true,
	tabSize: 2,
	minimap: {
		enabled: false,
	},
};

const defaultFilterRequestBody = `{
		"id": "Sample-1",
		"name": "Sample 1",
		"description": "This is sample data #1"
}`;

const defaultTransformFunctionContent = `/* 1. While you can write multiple functions, the main function
    called for your transformation is the transform function.

2. The only argument acceptable in the transform function is
    the payload data.

3. The transform method must return a value.

4. Console logs must be written like this
    console.log('%j', logged_item) to get printed in the log below. */

function transform(payload) {
    // Transform function here
    return payload; 
}`;

function CreateSubscriptionPage() {
	const { project } = useProjectStore();
	const { projectId } = Route.useParams();
	const {
		canManageSubscriptions,
		existingSources,
		existingEndpoints,
		hasAdvancedEndpointManagement,
		hasAdvancedSubscriptions,
		hasWebhookTransformations,
	} = Route.useLoaderData();
	const [toUseExistingSource, setToUseExistingSource] = useState(true);
	const [toUseExistingEndpoint, setToUseExistingEndpoint] = useState(true);
	const [showCustomResponse, setShowCustomResponse] = useState(false);
	const [showIdempotency, setShowIdempotency] = useState(false);
	const [showEventsFilterDialog, setShowEventsFilterDialog] = useState(false);
	const [showTransformFunctionDialog, setShowTransformFunctionDialog] =
		useState(false);
	const [hasPassedTestFilter, setHasPassedTestFilter] = useState(false);
	const [eventsFilter, setEventsFilter] = useState({
		request: {
			body: defaultFilterRequestBody,
			header: '',
		},
		schema: {
			body: `null`,
			header: `null`,
		},
	});

	const sourceVerifications = [
		{ uid: 'noop', name: 'None' },
		{ uid: 'hmac', name: 'HMAC' },
		{ uid: 'basic_auth', name: 'Basic Auth' },
		{ uid: 'api_key', name: 'API Key' },
		{ uid: 'github', name: 'Github' },
		{ uid: 'shopify', name: 'Shopify' },
		{ uid: 'twitter', name: 'Twitter' },
	];

	const form = useForm<z.infer<typeof CreateSubscriptionFormSchema>>({
		resolver: zodResolver(CreateSubscriptionFormSchema),
		defaultValues: {
			name: '',
			function: '',
			filter_config: {
				filter: {
					body: {},
					headers: {},
				},
				event_types: [],
			},
			source_id: '',
			source: {
				name: '',
				type: 'noop',
				config: {
					hash: '',
					encoding: '',
					header: '',
					secret: '',
					username: '',
					password: '',
					header_name: '',
					header_value: '',
				},
				custom_response: {
					content_type: '',
					body: '',
				},
				idempotency_keys: [],
			},
			endpoint_id: '',
			endpoint: {
				name: '',
				url: '',
				secret: '',
				owner_id: '',
				http_timeout: '',
				rate_limit: '',
				rate_limit_duration: '',
				support_email: '',
				slack_webhook_url: '',
				authentication: {
					type: 'api_key',
					api_key: {
						header_name: '',
						header_value: '',
					},
				},
				advanced_signatures: 'true',
				showHttpTimeout: false,
				showRateLimit: false,
				showOwnerId: false,
				showAuth: false,
				showNotifications: false,
				showSignatureFormat: false,
			},
			showTransform: false,
			showEventTypes: false,
			showEventsFilter: hasAdvancedSubscriptions,
		},
		mode: 'onTouched',
	});
	console.log(
		canManageSubscriptions,
		existingSources,
		form.formState.errors,
		form.getValues(),
	);

	function toggleUseExistingSource() {
		setToUseExistingSource(prev => !prev);
	}

	function toggleUseExistingEndpoint() {
		setToUseExistingEndpoint(prev => !prev);
	}

	async function testFilter() {
		setHasPassedTestFilter(false);
		try {
			const eventFilter = {
				request: {
					body: stringToJson(eventsFilter.request.body),
					header: stringToJson(eventsFilter.request.header),
				},
				schema: {
					body: stringToJson(eventsFilter.schema.body),
					header: stringToJson(eventsFilter.schema.header),
				},
			};
			const isPass = await subscriptionsService.testFilter(eventFilter);
			setHasPassedTestFilter(isPass);
			return eventFilter;
			// TODO show notification
		} catch (error) {
			console.error(error);
			return null;
		}
	}

	async function setSubscriptionFilter() {
		const eventFilter = await testFilter();
		if (hasPassedTestFilter && eventFilter) {
			const { schema } = eventFilter;
			form.setValue('filter_config.filter.body', schema.body);
			form.setValue('filter_config.filter.headers', schema.header);
		}
	}

	type SourceType =
		| 'noop'
		| 'hmac'
		| 'basic_auth'
		| 'api_key'
		| 'github'
		| 'shopify'
		| 'twitter';

	return (
		<DashboardLayout showSidebar={false}>
			<div className="w-full px-4 py-6">
				<div className="max-w-[770px] mx-auto">
					<div className="flex items-center mb-6">
						<Button
							variant="ghost"
							size="sm"
							asChild
							className="px-2 py-0 mr-2 bg-new.primary-25 rounded-8px"
						>
							<Link
								to="/projects/$projectId/subscriptions"
								params={{ projectId }}
								activeProps={{}}
							>
								<img
									src={modalCloseIcon}
									alt="Go back to subscriptions list"
									className="h-3 w-3"
								/>
							</Link>
						</Button>
						<h1 className="font-semibold text-sm capitalize">
							Create Subscription
						</h1>
					</div>

					<Form {...form}>
						<form className="flex flex-col gap-y-10">
							{/* Source */}
							{project?.type == 'incoming' && (
								<section>
									<h2 className="font-semibold text-sm">Source</h2>
									<p className="text-xs text-neutral-10 mt-1.5">
										Incoming event source this subscription is connected to.
									</p>

									<div className="border border-neutral-4 p-6 rounded-8px mt-6">
										{toUseExistingSource ? (
											<div>
												<FormField
													control={form.control}
													name="source_id"
													render={({ field }) => (
														<FormItem className="flex flex-col gap-y-2">
															<FormLabel className="text-neutral-9 text-xs">
																Select from existing sources
															</FormLabel>
															<Popover>
																<PopoverTrigger asChild className="shadow-none">
																	<FormControl>
																		<Button
																			variant="outline"
																			role="combobox"
																			className={cn(
																				'justify-end items-center',
																				!field.value && 'text-muted-foreground',
																			)}
																		>
																			{field.value
																				? existingSources.find(
																						source =>
																							source.uid === field.value,
																					)?.name
																				: ''}
																			<ChevronDown className="opacity-50" />
																		</Button>
																	</FormControl>
																</PopoverTrigger>
																<PopoverContent
																	align="start"
																	className="p-0 shadow-none"
																>
																	<Command className="shadow-none">
																		<CommandInput
																			placeholder="Filter source..."
																			className="h-9"
																		/>
																		<CommandList className="max-h-40">
																			<CommandEmpty>
																				No sources found.
																			</CommandEmpty>
																			<CommandGroup>
																				{existingSources.map(source => (
																					<CommandItem
																						className="cursor-pointer"
																						value={source.uid}
																						key={source.uid}
																						onSelect={() => {
																							form.setValue(
																								'source_id',
																								source.uid,
																							);
																						}}
																					>
																						{source.name}
																						<Check
																							className={cn(
																								'ml-auto',
																								source.uid === field.value
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
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<div className="mt-4">
													<Button
														disabled={!canManageSubscriptions}
														variant="ghost"
														size="sm"
														className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
														onClick={toggleUseExistingSource}
													>
														Create New Source
													</Button>
												</div>
											</div>
										) : (
											<div className="space-y-4">
												<h3 className="font-semibold mb-5 text-xs text-neutral-10">
													Pre-configured Sources
												</h3>
												<div className="flex flex-col gap-y-2">
													<ToggleGroup
														type="single"
														className="flex justify-start items-center gap-x-4"
														value={form.getValues('source.type')}
														onValueChange={(v: SourceType) =>
															form.setValue('source.type', v)
														}
													>
														<ToggleGroupItem
															value="github"
															aria-label="Toggle github"
															className={cn(
																'w-[60px] h-[60px] border border-neutral-6 px-4 py-[6px] rounded-8px hover:bg-white-100 !bg-white-100',
																form.watch('source.type') === 'github' &&
																	'!bg-white-100 border-new.primary-400',
															)}
														>
															<img
																src={githubIcon}
																alt="github preconfigured source"
															/>
														</ToggleGroupItem>
														<ToggleGroupItem
															value="shopify"
															aria-label="Toggle shopify"
															className={cn(
																'w-[60px] h-[60px] border border-neutral-6 px-4 py-[6px] rounded-8px hover:bg-white-100 !bg-white-100',
																form.watch('source.type') === 'shopify' &&
																	'!bg-white-100 border-new.primary-400',
															)}
														>
															<img
																src={shopifyIcon}
																alt="shopify preconfigured source"
															/>
														</ToggleGroupItem>
														<ToggleGroupItem
															value="twitter"
															aria-label="Toggle twitter"
															className={cn(
																'w-[60px] h-[60px] border border-neutral-6 px-4 py-[6px] rounded-8px hover:bg-white-100 !bg-white-100',
																form.watch('source.type') === 'twitter' &&
																	'!bg-white-100 border-new.primary-400',
															)}
														>
															<img
																src={twitterIcon}
																alt="twitter preconfigured source"
															/>
														</ToggleGroupItem>
													</ToggleGroup>
												</div>

												<hr />

												<div>
													<FormField
														name="source.name"
														control={form.control}
														render={({ field, fieldState }) => (
															<FormItem className="space-y-2">
																<FormLabel className="text-neutral-9 text-xs">
																	Source Name
																</FormLabel>
																<FormControl>
																	<Input
																		type="text"
																		autoComplete="text"
																		placeholder="Enter source name"
																		{...field}
																		className={cn(
																			'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																			fieldState.error
																				? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																				: ' hover:border-new.primary-100 focus:border-new.primary-300',
																		)}
																	></Input>
																</FormControl>
																<FormMessageWithErrorIcon />
															</FormItem>
														)}
													/>
												</div>

												<div>
													<FormField
														name="source.type"
														control={form.control}
														render={({ field }) => (
															<FormItem className="space-y-2">
																<FormLabel className="text-neutral-9 text-xs">
																	Source Verification
																</FormLabel>
																<Select
																	onValueChange={field.onChange}
																	defaultValue={field.value}
																>
																	<FormControl>
																		<SelectTrigger className="shadow-none">
																			<SelectValue placeholder="" />
																		</SelectTrigger>
																	</FormControl>
																	<SelectContent className="shadow-none">
																		{sourceVerifications.map(sv => (
																			<SelectItem
																				className="cursor-pointer"
																				value={sv.uid}
																				key={sv.uid}
																			>
																				{sv.name}
																			</SelectItem>
																		))}
																	</SelectContent>
																</Select>
															</FormItem>
														)}
													/>
												</div>

												{/* When source verification is HMAC */}
												{form.watch('source.type') == 'hmac' && (
													<div className="grid grid-cols-2 gap-x-5 gap-y-4">
														<p className="capitalize font-semibold text-xs col-span-full mt-4 text-neutral-10">
															Configure HMAC
														</p>

														<FormField
															name="source.config.encoding"
															control={form.control}
															render={({ field }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Encoding
																	</FormLabel>
																	<Select
																		onValueChange={field.onChange}
																		defaultValue={field.value}
																	>
																		<FormControl>
																			<SelectTrigger className="shadow-none">
																				<SelectValue placeholder="" />
																			</SelectTrigger>
																		</FormControl>
																		<SelectContent className="shadow-none">
																			{['base64', 'hex'].map(encoding => (
																				<SelectItem
																					className="cursor-pointer"
																					value={encoding}
																					key={encoding}
																				>
																					{encoding}
																				</SelectItem>
																			))}
																		</SelectContent>
																	</Select>
																</FormItem>
															)}
														/>

														<FormField
															name="source.config.hash"
															control={form.control}
															render={({ field }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Hash Algorithm
																	</FormLabel>
																	<Select
																		onValueChange={field.onChange}
																		defaultValue={field.value}
																	>
																		<FormControl>
																			<SelectTrigger className="shadow-none">
																				<SelectValue placeholder="" />
																			</SelectTrigger>
																		</FormControl>
																		<SelectContent className="shadow-none">
																			{['SHA256', 'SHA512'].map(hash => (
																				<SelectItem
																					className="cursor-pointer"
																					value={hash}
																					key={hash}
																				>
																					{hash}
																				</SelectItem>
																			))}
																		</SelectContent>
																	</Select>
																</FormItem>
															)}
														/>

														<FormField
															name="source.config.header"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Header Value
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="text"
																			autoComplete="text"
																			placeholder="Key goes here"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		></Input>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>

														<FormField
															name="source.config.secret"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Webhook signing secret
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="text"
																			placeholder="Secret goes here"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		></Input>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												)}

												{/* When source verification is basic auth */}
												{form.watch('source.type') == 'basic_auth' && (
													<div className="grid grid-cols-2 gap-x-5 gap-y-4">
														<p className="capitalize font-semibold text-xs col-span-full mt-4 text-neutral-10">
															Configure Basic Auth
														</p>

														<FormField
															name="source.config.username"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Username
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="text"
																			autoComplete="text"
																			placeholder="Username goes here"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		></Input>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>

														<FormField
															name="source.config.password"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Password
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="password"
																			autoComplete="text"
																			placeholder="********"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		></Input>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												)}

												{/* When source verification is API Key */}
												{form.watch('source.type') == 'api_key' && (
													<div className="grid grid-cols-2 gap-x-5 gap-y-4">
														<p className="capitalize font-semibold text-xs col-span-full mt-4 text-neutral-10">
															Configure API Key
														</p>

														<FormField
															name="source.config.header_name"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Header Name
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="text"
																			autoComplete="text"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		></Input>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>

														<FormField
															name="source.config.header_value"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Header Value
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="text"
																			autoComplete="text"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		></Input>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												)}

												{/* When source verification is github, twitter or shopify */}
												{['github', 'shopify', 'twitter'].includes(
													form.watch('source.type'),
												) && (
													<div className="grid grid-cols-1 gap-x-5 gap-y-4">
														<p className="capitalize font-semibold text-xs col-span-full mt-4 text-neutral-10">
															{form.watch('source.type')} Credentials
														</p>

														<FormField
															name="source.config.secret"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Webhook Signing Secret
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="text"
																			placeholder="Secret goes here"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		></Input>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												)}

												<div className="py-6">
													<hr />
												</div>

												{/* Checkboxes for custom response and idempotency */}
												<div className="flex gap-x-4">
													<label className="flex items-center gap-2 cursor-pointer">
														<div className="relative">
															<input
																type="checkbox"
																className=" peer
    appearance-none w-[14px] h-[14px] border-[1px] border-new.primary-300 rounded-sm bg-white-100
    mt-1 shrink-0 checked:bg-new.primary-300
     checked:border-0 cursor-pointer"
																checked={showCustomResponse}
																defaultChecked={false}
																onChange={e =>
																	// TODO if is false, reset values
																	setShowCustomResponse(e.target.checked)
																}
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
														<span className="block text-neutral-9 text-xs">
															Custom Response
														</span>
													</label>

													<label className="flex items-center gap-2 cursor-pointer">
														<div className="relative">
															<input
																type="checkbox"
																className=" peer
    appearance-none w-[14px] h-[14px] border-[1px] border-new.primary-300 rounded-sm bg-white-100
    mt-1 shrink-0 checked:bg-new.primary-300
     checked:border-0 cursor-pointer"
																checked={showIdempotency}
																defaultChecked={false}
																onChange={e =>
																	// TODO if is false, reset values
																	setShowIdempotency(e.target.checked)
																}
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
														<span className="block text-neutral-9 text-xs">
															Idempotency
														</span>
													</label>
												</div>

												{/* Custom Response */}
												{showCustomResponse && (
													<div className="border-l-2 border-new.primary-25 pl-4 flex flex-col gap-y-4">
														<h3 className="text-xs text-neutral-10 font-semibold">
															Custom Response
														</h3>

														<FormField
															name="source.custom_response.content_type"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Response Content Type
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="text"
																			placeholder="application/json, text/plain"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		></Input>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>

														<FormField
															name="source.custom_response.body"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Response Content
																	</FormLabel>
																	<FormControl>
																		<Textarea
																			placeholder="application/json, text/plain"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		/>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												)}

												{/* Idempotency */}
												{showIdempotency && (
													<div className="border-l border-new.primary-25 pl-4 flex flex-col gap-y-4">
														<h3 className="text-xs text-neutral-10 font-semibold">
															Idempotency Config
														</h3>

														<FormField
															name="source.idempotency_keys"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="flex items-center gap-x-1">
																		<span className="text-neutral-9 text-xs">
																			Key locations
																		</span>

																		<Tooltip>
																			<TooltipTrigger asChild>
																				<Info
																					size={12}
																					className="ml-1 text-neutral-9 inline"
																				/>
																			</TooltipTrigger>
																			<TooltipContent className="bg-white-100 border border-neutral-4">
																				<p className="w-[300px] text-xs text-neutral-10">
																					JSON location of idempotency key, add
																					multiple locations for different
																					locations
																				</p>
																			</TooltipContent>
																		</Tooltip>
																	</FormLabel>
																	<FormControl>
																		<InputTags
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																			name={field.name}
																			onChange={field.onChange}
																			value={field.value || []}
																		/>
																	</FormControl>
																	<p className="text-[10px] text-neutral-10 italic">
																		The order matters. Set the value of each
																		input with a coma (,)
																	</p>
																</FormItem>
															)}
														/>
													</div>
												)}

												<div className="mt-4">
													<Button
														type="button"
														variant="ghost"
														size="sm"
														className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
														onClick={toggleUseExistingSource}
													>
														Use Existing Source
													</Button>
												</div>
											</div>
										)}
									</div>
								</section>
							)}

							{/* Endpoint */}
							<section>
								<h2 className="font-semibold text-sm">Endpoint</h2>
								<p className="text-xs text-neutral-10 mt-1.5">
									Endpoint this subscription routes events into.
								</p>
								<div className="border border-neutral-4 p-6 rounded-8px mt-6">
									{toUseExistingEndpoint ? (
										<div>
											<FormField
												control={form.control}
												name="source_id"
												render={({ field }) => (
													<FormItem className="flex flex-col gap-y-2">
														<FormLabel className="text-neutral-9 text-xs">
															Select from existing endpoints
														</FormLabel>
														<Popover>
															<PopoverTrigger asChild className="shadow-none">
																<FormControl>
																	<Button
																		variant="outline"
																		role="combobox"
																		className={cn(
																			'justify-end items-center',
																			!field.value && 'text-muted-foreground',
																		)}
																	>
																		{field.value
																			? existingEndpoints.find(
																					ep => ep.uid === field.value,
																				)?.name
																			: ''}
																		<ChevronDown className="opacity-50" />
																	</Button>
																</FormControl>
															</PopoverTrigger>
															<PopoverContent
																align="start"
																className="p-0 shadow-none"
															>
																<Command className="shadow-none">
																	<CommandInput
																		placeholder="Filter endpoints..."
																		className="h-9"
																	/>
																	<CommandList className="max-h-40">
																		<CommandEmpty>
																			No sources found.
																		</CommandEmpty>
																		<CommandGroup>
																			{existingEndpoints.map(ep => (
																				<CommandItem
																					className="cursor-pointer"
																					value={ep.uid}
																					key={ep.uid}
																					onSelect={() => {
																						form.setValue(
																							'endpoint_id',
																							ep.uid,
																						);
																					}}
																				>
																					{ep.name}
																					<Check
																						className={cn(
																							'ml-auto',
																							ep.uid === field.value
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
														<FormMessageWithErrorIcon />
													</FormItem>
												)}
											/>

											<div className="mt-4">
												<Button
													disabled={!canManageSubscriptions}
													type="button"
													variant="ghost"
													size="sm"
													className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
													onClick={toggleUseExistingEndpoint}
												>
													Create New Endpoint
												</Button>
											</div>
										</div>
									) : (
										<div className="space-y-4">
											<div className="grid grid-cols-2 gap-x-5 gap-y-4">
												<FormField
													name="endpoint.name"
													control={form.control}
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-neutral-9 text-xs">
																Name
															</FormLabel>
															<FormControl>
																<Input
																	type="text"
																	autoComplete="text"
																	{...field}
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																></Input>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													name="endpoint.url"
													control={form.control}
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2">
															<FormLabel className="text-neutral-9 text-xs">
																URL
															</FormLabel>
															<FormControl>
																<Input
																	type="url"
																	autoComplete="text"
																	{...field}
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																></Input>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>

												<FormField
													name="endpoint.secret"
													control={form.control}
													render={({ field, fieldState }) => (
														<FormItem className="space-y-2 col-span-full">
															<FormLabel className="text-neutral-9 text-xs">
																Secret
															</FormLabel>
															<FormControl>
																<Input
																	type="text"
																	{...field}
																	className={cn(
																		'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																		fieldState.error
																			? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																			: ' hover:border-new.primary-100 focus:border-new.primary-300',
																	)}
																/>
															</FormControl>
															<FormMessageWithErrorIcon />
														</FormItem>
													)}
												/>
											</div>

											<div className="flex items-center gap-x-6">
												<label className="flex items-center gap-2 cursor-pointer">
													{/* TODO you may want to make this label into a component */}
													{/* TODO add popover to show if business and disabled */}
													<FormField
														control={form.control}
														name="endpoint.showHttpTimeout"
														render={({ field }) => (
															<FormItem>
																<FormControl className="relative">
																	<div className="relative">
																		<input
																			type="checkbox"
																			className=" peer
    appearance-none w-[14px] h-[14px] border-[1px] border-new.primary-300 rounded-sm bg-white-100
    mt-1 shrink-0 checked:bg-new.primary-300
     checked:border-0 cursor-pointer disabled:opacity-50"
																			defaultChecked={field.value}
																			onChange={field.onChange}
																			disabled={!hasAdvancedEndpointManagement}
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
															!hasAdvancedEndpointManagement && 'opacity-50',
														)}
													>
														Timeout
													</span>
												</label>

												<label className="flex items-center gap-2 cursor-pointer">
													{/* TODO you may want to make this label into a component */}
													{/* TODO add popover to show if business and disabled */}
													<FormField
														control={form.control}
														name="endpoint.showOwnerId"
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
																			defaultChecked={field.value}
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
														Owner ID
													</span>
												</label>

												<label className="flex items-center gap-2 cursor-pointer">
													{/* TODO you may want to make this label into a component */}
													{/* TODO add popover to show if business and disabled */}
													<FormField
														control={form.control}
														name="endpoint.showRateLimit"
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
																			defaultChecked={field.value}
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
													{/* TODO you may want to make this label into a component */}
													<FormField
														control={form.control}
														name="endpoint.showAuth"
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
																			defaultChecked={field.value}
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
														Auth
													</span>
												</label>

												<label className="flex items-center gap-2 cursor-pointer">
													{/* TODO you may want to make this label into a component */}
													{/* TODO add popover to show if business and disabled */}
													<FormField
														control={form.control}
														name="endpoint.showNotifications"
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
																			defaultChecked={field.value}
																			onChange={field.onChange}
																			disabled={!hasAdvancedEndpointManagement}
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
														Notifications
													</span>
												</label>

												{project?.type == 'outgoing' && (
													<label className="flex items-center gap-2 cursor-pointer">
														{/* TODO you may want to make this label into a component */}
														{/* TODO add popover to show if business and disabled */}
														<FormField
															control={form.control}
															name="endpoint.showSignatureFormat"
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
																				defaultChecked={field.value}
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
															Signature Format
														</span>
													</label>
												)}
											</div>

											{/* HTTP Timeout Section */}
											<div>
												{form.watch('endpoint.showHttpTimeout') && (
													<div className="pl-4 border-l-2 border-l-new.primary-25">
														<FormField
															control={form.control}
															name="endpoint.http_timeout"
															render={({ field, fieldState }) => (
																<FormItem className="w-full relative mb-2 block">
																	<div className="w-full mb-2 flex items-center justify-between">
																		<FormLabel
																			className="text-xs/5 text-neutral-9"
																			htmlFor="endpoint_http_timeout"
																		>
																			Timeout Value
																		</FormLabel>
																	</div>
																	<FormControl className="w-full relative">
																		<div className="relative">
																			<Input
																				id="endpoint_http_timeout"
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
																				placeholder="e.g 60"
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
													</div>
												)}
											</div>
											{/* Owner ID Section */}
											<div>
												{form.watch('endpoint.showOwnerId') && (
													<div className="pl-4 border-l-2 border-l-new.primary-25">
														<FormField
															name="endpoint.owner_id"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Owner ID
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="text"
																			autoComplete="text"
																			{...field}
																			className={cn(
																				'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																				fieldState.error
																					? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																					: ' hover:border-new.primary-100 focus:border-new.primary-300',
																			)}
																		></Input>
																	</FormControl>
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												)}
											</div>
											{/* Rate Limit Section */}
											<div>
												{form.watch('endpoint.showRateLimit') && (
													<div className="pl-4 border-l-2 border-l-new.primary-25">
														<p className="text-xs text-neutral-11 font-medium mb-3">
															Rate Limit
														</p>
														<div className="grid grid-cols-2 gap-x-5">
															<FormField
																control={form.control}
																name="endpoint.rate_limit_duration"
																render={({ field, fieldState }) => (
																	<FormItem className="w-full relative space-y-2">
																		<FormLabel
																			className="text-xs/5 text-neutral-9"
																			htmlFor="endpoint_ratelimit_duration"
																		>
																			Duration
																		</FormLabel>
																		<FormControl className="w-full relative">
																			<div className="relative">
																				<Input
																					id="endpoint_ratelimit_duration"
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
																name="endpoint.rate_limit"
																control={form.control}
																render={({ field, fieldState }) => (
																	<FormItem className="space-y-2">
																		<FormLabel className="text-neutral-9 text-xs">
																			Limit
																		</FormLabel>
																		<FormControl>
																			<Input
																				placeholder="e.g 10"
																				inputMode="numeric"
																				pattern="\d*"
																				type="number"
																				min={0}
																				{...field}
																				className={cn(
																					'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																					fieldState.error
																						? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																						: ' hover:border-new.primary-100 focus:border-new.primary-300',
																				)}
																			></Input>
																		</FormControl>
																		<FormMessageWithErrorIcon />
																	</FormItem>
																)}
															/>
														</div>
													</div>
												)}
											</div>
											{/* Auth Section */}
											<div>
												{form.watch('endpoint.showAuth') && (
													<div className="pl-4 border-l-2 border-l-new.primary-25">
														<p className="text-xs text-neutral-11 font-medium mb-3">
															Endpoint Authentication
															{/* TODO show tooltip */}
														</p>
														<div className="grid grid-cols-2 gap-x-5">
															<FormField
																name="endpoint.authentication.api_key.header_name"
																control={form.control}
																render={({ field, fieldState }) => (
																	<FormItem className="space-y-2">
																		<FormLabel className="text-neutral-9 text-xs">
																			API Key Name
																		</FormLabel>
																		<FormControl>
																			<Input
																				type="text"
																				autoComplete="text"
																				{...field}
																				className={cn(
																					'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																					fieldState.error
																						? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																						: ' hover:border-new.primary-100 focus:border-new.primary-300',
																				)}
																			></Input>
																		</FormControl>
																		<FormMessageWithErrorIcon />
																	</FormItem>
																)}
															/>

															<FormField
																name="endpoint.authentication.api_key.header_value"
																control={form.control}
																render={({ field, fieldState }) => (
																	<FormItem className="space-y-2">
																		<FormLabel className="text-neutral-9 text-xs">
																			API Key Value
																		</FormLabel>
																		<FormControl>
																			<Input
																				type="text"
																				autoComplete="text"
																				{...field}
																				className={cn(
																					'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																					fieldState.error
																						? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																						: ' hover:border-new.primary-100 focus:border-new.primary-300',
																				)}
																			></Input>
																		</FormControl>
																		<FormMessageWithErrorIcon />
																	</FormItem>
																)}
															/>
														</div>
													</div>
												)}
											</div>
											{/* Notifications Section */}
											<div>
												{form.watch('endpoint.showNotifications') && (
													<div className="pl-4 border-l-2 border-l-new.primary-25">
														<p className="text-xs text-neutral-11 font-medium mb-3">
															Alert Configuration
															{/* TODO show tooltip */}
														</p>
														<div className="grid grid-cols-2 gap-x-5">
															<FormField
																name="endpoint.support_email"
																control={form.control}
																render={({ field, fieldState }) => (
																	<FormItem className="space-y-2">
																		<FormLabel className="text-neutral-9 text-xs">
																			Support Email (tooltip)
																		</FormLabel>
																		<FormControl>
																			<Input
																				type="email"
																				autoComplete="on"
																				disabled={
																					!hasAdvancedEndpointManagement
																				}
																				{...field}
																				className={cn(
																					'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																					fieldState.error
																						? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																						: ' hover:border-new.primary-100 focus:border-new.primary-300',
																				)}
																			/>
																		</FormControl>
																		<FormMessageWithErrorIcon />
																	</FormItem>
																)}
															/>

															<FormField
																name="endpoint.slack_webhook_url"
																control={form.control}
																render={({ field, fieldState }) => (
																	<FormItem className="space-y-2">
																		<FormLabel className="text-neutral-9 text-xs">
																			Slack Webhook URL (tooltip)
																		</FormLabel>
																		<FormControl>
																			<Input
																				type="url"
																				autoComplete="on"
																				disabled={
																					!hasAdvancedEndpointManagement
																				}
																				{...field}
																				className={cn(
																					'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																					fieldState.error
																						? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																						: ' hover:border-new.primary-100 focus:border-new.primary-300',
																				)}
																			></Input>
																		</FormControl>
																		<FormMessageWithErrorIcon />
																	</FormItem>
																)}
															/>
														</div>
													</div>
												)}
											</div>
											{/* Signature Format Section */}
											<div>
												{form.watch('endpoint.showSignatureFormat') && (
													<div className="pl-4 border-l-2 border-l-new.primary-25">
														<FormField
															control={form.control}
															name="endpoint.advanced_signatures"
															render={({ field }) => (
																<FormItem className="w-full relative mb-6 block">
																	<p className="text-xs/5 text-neutral-9 mb-2">
																		Signature Format (tooltip)
																	</p>
																	<div className="flex w-full gap-x-6">
																		{[
																			{ label: 'Simple', value: 'false' },
																			{ label: 'Advanced', value: 'true' },
																		].map(({ label, value }) => {
																			return (
																				<FormControl
																					className="w-full "
																					key={label}
																				>
																					<label
																						className={cn(
																							'cursor-pointer border border-primary-100 flex items-start gap-x-2 p-4 rounded-sm',
																							field.value === value
																								? 'border-new.primary-300 bg-[#FAFAFE]'
																								: 'border-neutral-5',
																						)}
																						htmlFor={`sig_format_${label}`}
																					>
																						<span className="sr-only">
																							{label}
																						</span>
																						<Input
																							type="radio"
																							id={`sig_format_${label}`}
																							{...field}
																							value={value}
																							className="shadow-none h-4 w-fit"
																						/>
																						<div className="flex flex-col gap-y-1">
																							<p className="w-full text-xs text-neutral-10 font-semibold capitalize">
																								{label}
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
													</div>
												)}
											</div>

											<div className="mt-4">
												<Button
													type="button"
													variant="ghost"
													size="sm"
													className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
													onClick={toggleUseExistingEndpoint}
												>
													Use Existing Endpoint
												</Button>
											</div>
										</div>
									)}
								</div>
							</section>

							{/* Webhook Subscription Configuration */}
							<section>
								<h2 className="font-semibold text-sm">
									Webhook Subscription Configuration
								</h2>
								<p className="text-xs text-neutral-10 mt-1.5">
									Configure how you want this endpoint to receive webhook
									events.
								</p>
								<div className="border border-neutral-4 p-6 rounded-8px mt-6">
									<div className="space-y-6">
										<FormField
											name="name"
											control={form.control}
											render={({ field, fieldState }) => (
												<FormItem className="space-y-2">
													<FormLabel className="text-neutral-9 text-xs">
														Subscription Name
													</FormLabel>
													<FormControl>
														<Input
															type="text"
															autoComplete="text"
															placeholder="e.g paystack-live"
															{...field}
															className={cn(
																'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
																fieldState.error
																	? 'border-destructive focus-visible:ring-0 hover:border-destructive'
																	: ' hover:border-new.primary-100 focus:border-new.primary-300',
															)}
														></Input>
													</FormControl>
													<FormMessageWithErrorIcon />
												</FormItem>
											)}
										/>

										<hr />

										<div className="flex gap-x-4 items-center">
											<label className="flex items-center gap-2 cursor-pointer">
												{/* TODO you may want to make this label into a component */}
												<FormField
													control={form.control}
													name="showEventsFilter"
													render={({ field }) => (
														<FormItem>
															<FormControl className="relative">
																<div className="relative">
																	<input
																		type="checkbox"
																		className=" peer
    appearance-none w-[14px] h-[14px] border-[1px] border-new.primary-300 rounded-sm bg-white-100
    mt-1 shrink-0 checked:bg-new.primary-300
     checked:border-0 cursor-pointer disabled:opacity-50"
																		defaultChecked={field.value}
																		onChange={field.onChange}
																		disabled={!hasAdvancedSubscriptions}
																		// TODO: for business icon
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
														!hasAdvancedSubscriptions && 'opacity-50',
													)}
												>
													Events Filter
												</span>
											</label>

											{project?.type == 'outgoing' && (
												<label className="flex items-center gap-2 cursor-pointer">
													{/* TODO you may want to make this label into a component */}
													<FormField
														control={form.control}
														name="showEventTypes"
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
																			defaultChecked={field.value}
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
														Event Types
													</span>
												</label>
											)}

											{project?.type == 'incoming' && (
												<label className="flex items-center gap-2 cursor-pointer">
													{/* TODO you may want to make this label into a component */}
													<FormField
														control={form.control}
														name="showTransform"
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
																			defaultChecked={field.value}
																			onChange={field.onChange}
																			disabled={!hasWebhookTransformations}
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
															!hasWebhookTransformations && 'opacity-50',
														)}
													>
														Transform
													</span>
												</label>
											)}
										</div>

										<div className="flex flex-col gap-y-6">
											{form.watch('showEventsFilter') && (
												<div className="pl-4 border-l-2 border-l-new.primary-25 flex justify-between items-center">
													<div className="flex flex-col gap-y-2 justify-center">
														<p className="text-neutral-10 font-medium text-xs">
															Events filter
														</p>
														<p className="text-[10px] text-neutral-10">
															Filter events received by request body and header
														</p>
													</div>
													<div>
														<Button
															type="button"
															variant="outline"
															size="sm"
															disabled={!hasAdvancedSubscriptions}
															className="text-xs text-neutral-10 shadow-none hover:text-neutral-10 hover:bg-white-100"
															onClick={e => {
																e.stopPropagation();
																setShowTransformFunctionDialog(false);
																setShowEventsFilterDialog(true);
															}}
														>
															Open Editor
														</Button>
													</div>
												</div>
											)}

											{form.watch('showTransform') && (
												<div className="pl-4 border-l-2 border-l-new.primary-25 flex justify-between items-center">
													<div className="flex flex-col gap-y-2 justify-center">
														<p className="text-neutral-10 font-medium text-xs">
															Transform
														</p>
														<p className="text-[10px] text-neutral-10">
															Transform request body of events with a JavaScript
															function.
														</p>
													</div>
													<div>
														<Button
															type="button"
															variant="outline"
															size="sm"
															disabled={!hasWebhookTransformations}
															className="text-xs text-neutral-10 shadow-none hover:text-neutral-10 hover:bg-white-100"
															onClick={e => {
																e.stopPropagation();
																setShowEventsFilterDialog(false);
																setShowTransformFunctionDialog(true);
															}}
														>
															Open Editor
														</Button>
													</div>
												</div>
											)}
										</div>
									</div>
								</div>
							</section>
						</form>
					</Form>
				</div>
			</div>

			{/* Events Filter Dialog */}
			<Dialog
				open={showEventsFilterDialog}
				onOpenChange={setShowEventsFilterDialog}
			>
				<DialogContent className="max-w-[1280px] w-[80vw] h-[80vh] rounded-md">
					<DialogHeader className="flex flex-row items-center justify-between w-[80%] mx-auto space-y-0">
						<DialogTitle className="font-semibold text-sm capitalize">
							Subscription Filter
						</DialogTitle>
						<DialogDescription className="sr-only">
							Subscription Filter
						</DialogDescription>
						<div className="flex items-center justify-end gap-x-4">
							<Button
								variant="outline"
								size="sm"
								className="shadow-none border-new.primary-400 hover:bg-white-100 text-new.primary-400 hover:text-new.primary-400"
								onClick={testFilter}
							>
								Test Filter
								<svg width="18" height="18" className="fill-white-100">
									<use xlinkHref="#test-icon"></use>
								</svg>
							</Button>
							<Button
								size="sm"
								variant="ghost"
								className="shadow-none bg-new.primary-400 text-white-100 hover:bg-new.primary-400 hover:text-white-100"
								disabled={!hasPassedTestFilter}
								onClick={setSubscriptionFilter}
							>
								Save
							</Button>
						</div>
					</DialogHeader>

					<div className="mt-10 border border-neutral-4 rounded-md">
						<Tabs
							defaultValue="body"
							// value={activeFilterTab}
							// onValueChange={setActiveFilterTab}
							className="w-full"
						>
							<TabsList className="flex justify-center w-full border-b-[.5px] border-neutral-4 rounded-none">
								<TabsTrigger value="body" className="capitalize" type="button">
									body
								</TabsTrigger>
								<TabsTrigger
									value="header"
									className="capitalize"
									type="button"
								>
									header
								</TabsTrigger>
							</TabsList>

							<TabsContent value="body" className="m-0">
								<div className="flex">
									<div className="flex flex-col w-full border-r border-r-neutral-4">
										<div className="text-sm border-b border-b-neutral-4 p-5 font-semibold">
											Event Payload
										</div>
										<div className="h-[350px] p-4">
											<Editor
												className="max-h-[100%]"
												defaultLanguage="json"
												defaultValue={eventsFilter.request.body}
												onChange={body =>
													setEventsFilter(prev => ({
														...prev,
														request: { ...prev.request, body: body || '' },
													}))
												}
												options={monacoEditorOptions}
											/>
										</div>
									</div>
									<div className="flex flex-col w-full">
										<div className="text-sm border-b border-b-neutral-4 p-5 font-semibold">
											Filter Schema
										</div>
										<div className="h-[400px] p-4">
											<Editor
												className="max-h-[100%]"
												defaultLanguage="json"
												defaultValue={eventsFilter.schema.body}
												onChange={body =>
													setEventsFilter(prev => ({
														...prev,
														schema: { ...prev.schema, body: body || '' },
													}))
												}
												options={monacoEditorOptions}
											/>
										</div>
									</div>
								</div>
							</TabsContent>

							<TabsContent value="header" className="m-0">
								<div className="flex">
									<div className="flex flex-col w-full border-r border-r-neutral-4">
										<div className="text-sm border-b border-b-neutral-4 p-5 font-semibold">
											Event Headers
										</div>
										<div className="h-[400px] p-4">
											<Editor
												className="max-h-[100%]"
												defaultLanguage="json"
												defaultValue={eventsFilter.request.header}
												onChange={header =>
													setEventsFilter(prev => ({
														...prev,
														request: { ...prev.request, header: header || '' },
													}))
												}
												options={monacoEditorOptions}
											/>
										</div>
									</div>
									<div className="flex flex-col w-full">
										<div className="text-sm border-b border-b-neutral-4 p-5 font-semibold">
											Filter Schema
										</div>
										<div className="h-[400px] p-4">
											<Editor
												className="max-h-[100%]"
												defaultLanguage="json"
												defaultValue={eventsFilter.schema.header}
												onChange={header =>
													setEventsFilter(prev => ({
														...prev,
														schema: { ...prev.schema, header: header || '' },
													}))
												}
												options={monacoEditorOptions}
											/>
										</div>
									</div>
								</div>
							</TabsContent>
						</Tabs>
					</div>
				</DialogContent>
			</Dialog>

			{/* Transform Function Dialog */}
			<Dialog
				open={showTransformFunctionDialog}
				// open={true}
				onOpenChange={setShowTransformFunctionDialog}
			>
				<DialogContent className="max-w-[90vw] h-[90vh] rounded-md grid grid-rows-10 grid-flow-col">
					<DialogHeader className="flex flex-row items-center justify-between w-[80%] mx-auto space-y-0 row-span-1">
						<DialogTitle className="font-semibold text-sm capitalize">
							Subscription Transform
						</DialogTitle>
						<DialogDescription className="sr-only">
							Subscription Transform
						</DialogDescription>

						<Button
							variant="outline"
							size="sm"
							className="shadow-none bg-new.primary-400 text-white-100 hover:bg-new.primary-400 hover:text-white-100"
							onClick={() => console.log('save fn')}
						>
							<Save className="stroke-white-100" />
							Save Function
						</Button>
					</DialogHeader>

					<div className="border border-neutral-4 rounded-md row-span-9 grid grid-cols-2">
						<div className="border border-neutral-4 grid grid-rows-2 max-h-[70vh]">
							<div className="flex flex-col">
								<div className="text-xs text-neutral-10 border-l border-neutral-4 p-2">
									Input
								</div>
								<div className="flex-1">
									<Editor
										defaultLanguage="json"
										defaultValue={defaultFilterRequestBody}
										options={monacoEditorOptions}
									/>
								</div>
							</div>
							<div>
								<Tabs defaultValue="output" className="w-full">
									<TabsList className="w-full p-0 bg-background justify-start border-b rounded-none">
										<TabsTrigger
											type="button"
											value="output"
											className="text-sm rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-primary"
										>
											Output
										</TabsTrigger>

										<TabsTrigger
											value="diff"
											className="text-sm rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-primary"
										>
											Diff
										</TabsTrigger>
									</TabsList>
									<TabsContent value="output">
										<div className="h-10 flex items-center justify-between border gap-2 rounded-md pl-3 pr-1.5">
											<code className="text-[13px]">Output</code>
										</div>
									</TabsContent>

									<TabsContent value="diff">
										<div className="h-10 flex items-center justify-between border gap-2 rounded-md pl-3 pr-1.5">
											<code className="text-[13px]">Diff</code>
										</div>
									</TabsContent>
								</Tabs>
							</div>
						</div>
						<div className="border border-neutral-4 grid grid-rows-2 max-h-[70vh]">
							<div>
								<div className="text-xs text-neutral-10 border border-neutral-4 p-1 flex justify-between items-center">
									<p>Transform Function</p>
									<Button
										variant={'outline'}
										size={'sm'}
										className="py-1 px-2 bg-white-100 hover:text-new.primary-400 text-xs hover:bg-white-100 border-new.primary-400 text-new.primary-400 shadow-none"
									>
										Run
									</Button>
								</div>
								<Editor
									defaultLanguage="javascript"
									defaultValue={defaultTransformFunctionContent}
									options={monacoEditorOptions}
								/>
							</div>
							<div className="">
								Lorem ipsum dolor sit, amet consectetur adipisicing elit. Veniam
								odio, dolores tempore dolor maxime eius fugiat ea voluptates
								enim nulla pariatur iste molestias beatae temporibus ipsa optio
								culpa quibusdam. Maiores.
							</div>
						</div>
					</div>
				</DialogContent>
			</Dialog>
		</DashboardLayout>
	);
}
