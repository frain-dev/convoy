import { z } from 'zod';
import { useState } from 'react';
import { Editor, DiffEditor } from '@monaco-editor/react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createFileRoute, Link } from '@tanstack/react-router';

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
	PopoverClose,
} from '@/components/ui/popover';
import { ConvoyCheckbox } from '@/components/convoy-checkbox';
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
import * as sourcesService from '@/services/sources.service';

import githubIcon from '../../../../../assets/img/github.png';
import shopifyIcon from '../../../../../assets/img/shopify.png';
import twitterIcon from '../../../../../assets/img/twitter.png';
import modalCloseIcon from '../../../../../assets/svg/modal-close-icon.svg';

type SourceType = (typeof sourceVerifications)[number]['uid'];

const sourceVerifications = [
	{ uid: 'noop', name: 'None' },
	{ uid: 'hmac', name: 'HMAC' },
	{ uid: 'basic_auth', name: 'Basic Auth' },
	{ uid: 'api_key', name: 'API Key' },
	{ uid: 'github', name: 'Github' },
	{ uid: 'shopify', name: 'Shopify' },
	{ uid: 'twitter', name: 'Twitter' },
] as const;

type FuncOutput = {
	previous: Record<string, unknown> | null | string;
	current: Record<string, unknown> | null | string;
};

const editorOptions = {
	formatOnType: true,
	formatOnPaste: true,
	minimap: { enabled: false },
	scrollBeyondLastLine: false,
	fontSize: 12,
};

const CreateSourceFormSchema = z
	.object({
		name: z.string().min(1, 'Enter new source name'),
		type: z.enum([
			sourceVerifications[0].uid,
			...sourceVerifications.slice(1).map(t => t.uid),
		]),
		is_disabled: z.boolean().optional(),
		config: z.object({
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
		showHmac: z.boolean(),
		showBasicAuth: z.boolean(),
		showAPIKey: z.boolean(),
		showGithub: z.boolean(),
		showTwitter: z.boolean(),
		showShopify: z.boolean(),
		showCustomResponse: z.boolean(),
		showIdempotency: z.boolean(),
	})
	.refine(
		v => {
			if (
				v.showCustomResponse &&
				(!v.custom_response?.content_type || !v.custom_response.body)
			) {
				return false;
			}
			return true;
		},
		({ custom_response }) => {
			if (!custom_response?.content_type)
				return {
					message: 'Enter content type',
					path: ['custom_response.content_type'],
				};

			if (!custom_response?.body)
				return {
					message: 'Enter response content',
					path: ['custom_response.body'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ showIdempotency, idempotency_keys }) => {
			if (showIdempotency && idempotency_keys?.length == 0) return false;
			return true;
		},
		() => {
			return {
				message:
					'Add at least one idempotency key if idempotency configuration is enabled',
				path: ['idempotency_keys'],
			};
		},
	)
	.refine(
		({ type, config }) => {
			const { encoding, hash, header, secret } = config;
			const hasInvalidValue = !encoding || !hash || !header || !secret;
			if (type == 'hmac' && hasInvalidValue) return false;
			return true;
		},
		({ config }) => {
			const { encoding, hash, header, secret } = config;
			if (!encoding)
				return {
					message: 'Enter encoding value',
					path: ['config.encoding'],
				};

			if (!hash)
				return {
					message: 'Enter hash value',
					path: ['config.hash'],
				};

			if (!header)
				return {
					message: 'Enter header key',
					path: ['config.header'],
				};

			if (!secret)
				return {
					message: 'Enter webhook signing secret',
					path: ['config.secret'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ type, config }) => {
			const { secret } = config;
			const isPreconfigured = ['github', 'shopify', 'twitter'].includes(type);
			if (isPreconfigured && !secret) return false;
			return true;
		},
		() => ({
			message: 'Enter webhook signing secret',
			path: ['config.secret'],
		}),
	)
	.refine(
		({ type, config }) => {
			const { username, password } = config;
			const hasInvalidValue = !username || !password;
			if (type == 'basic_auth' && hasInvalidValue) return false;
			return true;
		},
		({ config }) => {
			const { username, password } = config;
			if (!username)
				return {
					message: 'Enter username',
					path: ['config.username'],
				};

			if (!password)
				return {
					message: 'Enter password',
					path: ['config.password'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ type, config }) => {
			const { header_name, header_value } = config;
			const hasInvalidValue = !header_name || !header_value;
			if (type == 'api_key' && hasInvalidValue) return false;
			return true;
		},
		({ config }) => {
			const { header_name, header_value } = config;
			if (!header_name)
				return {
					message: 'Enter API header key',
					path: ['config.header_name'],
				};

			if (!header_value)
				return {
					message: 'Enter API header value',
					path: ['config.header_value'],
				};

			return { message: '', path: [] };
		},
	);

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
	showIdempotency: z.boolean(),
	showCustomResponse: z.boolean(),
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

const defaultOutput = { previous: '', current: '' };

const defaultFilterRequestBody = `{
		"id": "Sample-1",
		"name": "Sample 1",
		"description": "This is sample data #1"
}`;

const defaultBody = {
	id: 'Sample-1',
	name: 'Sample 1',
	description: 'This is sample data #1',
};

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
	const [showEventsFilterDialog, setShowEventsFilterDialog] = useState(false);
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

	const [showTransformFunctionDialog, setShowTransformFunctionDialog] =
		useState(false);
	// Transform function state variables
	// TODO use a reducer hook
	const [isTestingFunction, setIsTestingFunction] = useState(false);
	const [isTransformPassed, setIsTransformPassed] = useState(false);
	const [showConsole, setShowConsole] = useState(true);
	const [transformBodyPayload, setTransformBodyPayload] =
		useState<Record<string, unknown>>(defaultBody);
	const [headerPayload, setHeaderPayload] =
		useState<Record<string, unknown>>(defaultBody);
	const [transformFnBody, setTransformFnBody] = useState<string>(
		`/*  1. While you can write multiple functions, the main
	 function called for your transformation is the transform function.

2. The only argument acceptable in the transform function is the
 payload data.

3. The transform method must return a value.

4. Console logs lust be written like this:
console.log('%j', logged_item) to get printed in the log below.

5. The output payload from the function should be in this format
		{
				"owner_id": "string, optional",
				"event_type": "string, required",
				"data": "object, required",
				"custom_headers": "object, optional",
				"idempotency_key": "string, optional"
				"endpoint_id": "string, depends",
		}

6. The endpoint_id field is only required when sending event to
a single endpoint. */

function transform(payload) {
		// Transform function here
		return {
				"endpoint_id": "",
				"owner_id": "",
				"event_type": "sample",
				"data": payload,
				"custom_headers": {
						"sample-header": "sample-value"
				},
				"idempotency_key": ""
		}
}`,
	);
	const [transformFnHeader, setTransformFnHeader] = useState<string>(
		`/* 1. While you can write multiple functions, the main function
called for your transformation is the transform function.

2. The only argument acceptable in the transform function is
the payload data.

3. The transform method must return a value.

4. Console logs lust be written like this
console.log('%j', logged_item) to get printed in the log below. */

function transform(payload) {
// Transform function here
return payload;
}`,
	);
	const [bodyOutput, setBodyOutput] = useState<FuncOutput>(defaultOutput);
	const [headerOutput, setHeaderOutput] = useState<FuncOutput>(defaultOutput);
	const [bodyLogs, setBodyLogs] = useState<string[]>([]);
	const [headerLogs, setHeaderLogs] = useState<string[]>([]);
	const [, /* transformFn */ setTransformFn] = useState<string>();
	const [, /* headerTransformFn */ setHeaderTransformFn] = useState<string>();
	const [hasSavedFn, setHasSavedFn] = useState(false);
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
			showIdempotency: false,
			showCustomResponse: false,
		},
		mode: 'onTouched',
	});
	console.log(
		existingSources,
		CreateSubscriptionFormSchema.safeParse(form.getValues()),
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

	// Function to test transform
	async function testTransformFunction(type: 'body' | 'header') {
		setIsTransformPassed(false);
		setIsTestingFunction(true);

		const payload = type === 'body' ? transformBodyPayload : headerPayload;
		const transformFunc = type === 'body' ? transformFnBody : transformFnHeader;

		try {
			// Call the sources service to test the transform function
			const response = await sourcesService.testTransformFunction({
				payload,
				function: transformFunc,
				type,
			});

			// In a real implementation, this would return payload and logs
			if (type === 'body') {
				setBodyOutput(prev => ({
					current: response.payload,
					previous: prev.current,
				}));
				setBodyLogs(
					response.log.toReversed() || [
						'Transform function executed successfully',
					],
				);
			} else {
				setHeaderOutput(prev => ({
					current: response.payload,
					previous: prev.current,
				}));
				setHeaderLogs(
					response.log.toReversed() || [
						'Transform function executed successfully',
					],
				);
			}

			setIsTransformPassed(true);
			setIsTestingFunction(false);
			setShowConsole(bodyLogs.length || headerLogs.length ? true : false);

			if (type === 'body') {
				setTransformFn(transformFunc);
			} else {
				setHeaderTransformFn(transformFunc);
			}
		} catch (error) {
			console.error(error);
			setIsTestingFunction(false);
			if (type === 'body') {
				setBodyLogs(['Error executing transform function']);
			} else {
				setHeaderLogs(['Error executing transform function']);
			}
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
						<form className="flex flex-col gap-y-6">
							{/* Source */}
							{project?.type == 'incoming' && (
								<section>
									<h2 className="font-semibold text-sm">Source</h2>
									<p className="text-xs text-neutral-10 mt-1.5">
										Incoming event source this subscription is connected to.
									</p>

									<div className="border border-neutral-4 p-6 rounded-8px mt-6">
										{toUseExistingSource ? (
											<div className="space-y-4">
												<FormField
													control={form.control}
													name="source_id"
													render={({ field }) => (
														<FormItem className="flex flex-col gap-y-2">
															<FormLabel className="text-neutral-9 text-xs">
																Select from existing sources
															</FormLabel>
															<Popover>
																<PopoverTrigger
																	asChild
																	className="shadow-none h-12"
																>
																	<FormControl>
																		<Button
																			variant="outline"
																			role="combobox"
																			className="flex items-center text-xs text-neutral-10 hover:text-neutral-10"
																		>
																			{field.value
																				? existingSources.find(
																						source =>
																							source.uid === field.value,
																					)?.name
																				: ''}
																			<ChevronDown className="ml-auto opacity-50" />
																		</Button>
																	</FormControl>
																</PopoverTrigger>
																<PopoverContent
																	align="start"
																	className="p-0 shadow-none w-full"
																>
																	<Command className="shadow-none">
																		<CommandInput
																			placeholder="Filter source"
																			className="h-9"
																			onInput={e => {
																				form.setValue(
																					'source_id',
																					(e.target as HTMLInputElement).value,
																				);
																			}}
																		/>
																		<CommandList className="max-h-40">
																			<CommandEmpty className="text-xs text-neutral-10 hover:text-neutral-10 py-4">
																				No sources found.
																			</CommandEmpty>
																			<CommandGroup>
																				{existingSources.map(source => (
																					<PopoverClose
																						key={source.uid}
																						className="flex flex-col w-full"
																					>
																						<CommandItem
																							className="cursor-pointer text-xs !text-neutral-10 py-4 !hover:text-neutral-10"
																							value={`${source.name}-${source.uid}`}
																							onSelect={() => {
																								form.setValue(
																									'source_id',
																									source.uid,
																								);
																							}}
																						>
																							{source.name} ({source.uid})
																							<Check
																								className={cn(
																									'ml-auto',
																									source.uid === field.value
																										? 'opacity-100 stroke-neutral-10'
																										: 'opacity-0',
																								)}
																							/>
																						</CommandItem>
																					</PopoverClose>
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

												<div>
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
											<div className="grid grid-cols-1 w-full gap-y-4">
												<h3 className="font-semibold text-xs text-neutral-10">
													Pre-configured Sources
												</h3>
												<div className="flex flex-col gap-y-2">
													<ToggleGroup
														type="single"
														className="flex justify-start items-center gap-x-4"
														value={form.watch('source.type')}
														onValueChange={(v: SourceType) => {
															form.setValue('source.type', v);
															form.setValue(
																'source.name',
																`${v.charAt(0).toUpperCase()}${v.slice(1)} Source`,
															);
														}}
													>
														<ToggleGroupItem
															value="github"
															aria-label="Toggle github"
															className={cn(
																'w-[60px] h-[60px] border border-neutral-6 px-4 py-[6px] rounded-8px hover:bg-white-100 !bg-white-100',
																form.watch('source.type') === 'github' &&
																	'border-new.primary-400',
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
																	'border-new.primary-400',
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
																	'border-new.primary-400',
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
																	value={form.watch('source.type')}
																	onValueChange={(v: SourceType) => {
																		field.onChange(v);
																		if (
																			['github', 'shopify', 'twitter'].includes(
																				v,
																			)
																		) {
																			form.setValue(
																				'source.name',
																				`${v.charAt(0).toUpperCase()}${v.slice(1)} Source`,
																			);
																		}
																	}}
																	defaultValue={field.value}
																>
																	<FormControl>
																		<SelectTrigger className="shadow-none h-12 focus:ring-0 text-neutral-9 text-xs">
																			<SelectValue
																				placeholder=""
																				className="text-xs text-neutral-10"
																			/>
																		</SelectTrigger>
																	</FormControl>
																	<SelectContent className="shadow-none">
																		{sourceVerifications.map(sv => (
																			<SelectItem
																				className="cursor-pointer text-xs py-3 hover:bg-transparent"
																				value={sv.uid}
																				key={sv.uid}
																			>
																				<span className="text-neutral-10">
																					{sv.name}
																				</span>
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
														<h4 className="capitalize font-semibold text-xs col-span-full text-neutral-10">
															Configure HMAC
														</h4>

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
																			<SelectTrigger className="shadow-none h-12 focus:ring-0 text-neutral-9 text-xs">
																				<SelectValue
																					placeholder=""
																					className="text-xs text-neutral-10"
																				/>
																			</SelectTrigger>
																		</FormControl>
																		<SelectContent className="shadow-none">
																			{['base64', 'hex'].map(encoding => (
																				<SelectItem
																					className="cursor-pointer text-xs py-3 hover:bg-transparent"
																					value={encoding}
																					key={encoding}
																				>
																					<span className="text-neutral-10">
																						{encoding}
																					</span>
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
																			<SelectTrigger className="shadow-none h-12 focus:ring-0 text-neutral-9 text-xs">
																				<SelectValue
																					placeholder=""
																					className="text-xs text-neutral-10"
																				/>
																			</SelectTrigger>
																		</FormControl>
																		<SelectContent className="shadow-none">
																			{['SHA256', 'SHA512'].map(hash => (
																				<SelectItem
																					className="cursor-pointer text-xs py-3 hover:bg-transparent"
																					value={hash}
																					key={hash}
																				>
																					{' '}
																					<span className="text-neutral-10">
																						{hash}
																					</span>
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
														<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
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
																			autoComplete="off"
																			placeholder="Username goes here"
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
															name="source.config.password"
															control={form.control}
															render={({ field, fieldState }) => (
																<FormItem className="space-y-2">
																	<FormLabel className="text-neutral-9 text-xs">
																		Password
																	</FormLabel>
																	<FormControl>
																		<Input
																			type="text"
																			autoComplete="off"
																			placeholder="********"
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

												{/* When source verification is API Key */}
												{form.watch('source.type') == 'api_key' && (
													<div className="grid grid-cols-2 gap-x-5 gap-y-4">
														<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
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
																		/>
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
														<p className="capitalize font-semibold text-xs col-span-full text-neutral-10">
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

												<div className="py-3">
													<hr />
												</div>

												{/* Checkboxes for custom response and idempotency */}
												<div className="flex items-center gap-x-6">
													<FormField
														control={form.control}
														name="showCustomResponse"
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<ConvoyCheckbox
																		label="Custom Response"
																		isChecked={field.value}
																		onChange={field.onChange}
																	/>
																</FormControl>
															</FormItem>
														)}
													/>

													<FormField
														control={form.control}
														name="showIdempotency"
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<ConvoyCheckbox
																		label="Idempotency"
																		isChecked={field.value}
																		onChange={field.onChange}
																	/>
																</FormControl>
															</FormItem>
														)}
													/>
												</div>

												{/* Custom Response */}
												{form.watch('showCustomResponse') && (
													<div className="border-l border-new.primary-25 pl-4 flex flex-col gap-y-4">
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
																		/>
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
																			rows={6}
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
												{form.watch('showIdempotency') && (
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
																			<TooltipTrigger
																				asChild
																				className="hover:cursor-pointer"
																			>
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
																	<FormMessageWithErrorIcon />
																</FormItem>
															)}
														/>
													</div>
												)}

												<div>
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
															<PopoverTrigger
																asChild
																className="shadow-none h-12"
															>
																<FormControl>
																	<Button
																		variant="outline"
																		role="combobox"
																		className="flex items-center text-xs text-neutral-10 hover:text-neutral-10"
																	>
																		{field.value
																			? existingEndpoints.find(
																					ep => ep.uid === field.value,
																				)?.name
																			: ''}
																		<ChevronDown className="ml-auto opacity-50" />
																	</Button>
																</FormControl>
															</PopoverTrigger>
															<PopoverContent
																align="start"
																className="p-0 shadow-none w-full"
															>
																<Command className="shadow-none">
																	<CommandInput
																		placeholder="Filter endpoints"
																		className="h-9"
																		onInput={e => {
																			form.setValue(
																				'source_id',
																				(e.target as HTMLInputElement).value,
																			);
																		}}
																	/>
																	<CommandList className="max-h-40">
																		<CommandEmpty className="text-xs text-neutral-10 hover:text-neutral-10 py-4">
																			No endpoints found.
																		</CommandEmpty>
																		<CommandGroup>
																			{existingEndpoints.map(ep => (
																				<PopoverClose
																					key={ep.uid}
																					className="flex flex-col w-full"
																				>
																					<CommandItem
																						className="cursor-pointer text-xs !text-neutral-10 py-4 !hover:text-neutral-10"
																						value={`${ep.name}-${ep.uid}`}
																						onSelect={() => {
																							form.setValue(
																								'endpoint_id',
																								ep.uid,
																							);
																						}}
																					>
																						{ep.name} ({ep.uid})
																						<Check
																							className={cn(
																								'ml-auto',
																								ep.uid === field.value
																									? 'opacity-100 stroke-neutral-10'
																									: 'opacity-0',
																							)}
																						/>
																					</CommandItem>
																				</PopoverClose>
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
												<FormField
													control={form.control}
													name="endpoint.showHttpTimeout"
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	label="Timeout"
																	isChecked={field.value}
																	onChange={field.onChange}
																	disabled={!hasAdvancedEndpointManagement}
																/>
															</FormControl>
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="endpoint.showOwnerId"
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	label="Owner ID"
																	isChecked={field.value}
																	onChange={field.onChange}
																/>
															</FormControl>
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="endpoint.showRateLimit"
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	label="Rate Limit"
																	isChecked={field.value}
																	onChange={field.onChange}
																/>
															</FormControl>
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="endpoint.showAuth"
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	label="Auth"
																	isChecked={field.value}
																	onChange={field.onChange}
																/>
															</FormControl>
														</FormItem>
													)}
												/>

												<FormField
													control={form.control}
													name="endpoint.showNotifications"
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	label="Notifications"
																	isChecked={field.value}
																	onChange={field.onChange}
																	disabled={!hasAdvancedEndpointManagement}
																/>
															</FormControl>
														</FormItem>
													)}
												/>

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
													<div className="pl-4 border-l border-l-new.primary-25">
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
													<div className="pl-4 border-l border-l-new.primary-25">
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
													<div className="pl-4 border-l border-l-new.primary-25">
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
													<div className="pl-4 border-l border-l-new.primary-25">
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
													<div className="pl-4 border-l border-l-new.primary-25">
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
													<div className="pl-4 border-l border-l-new.primary-25">
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

											<div>
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
												<div className="pl-4 border-l border-l-new.primary-25 flex justify-between items-center">
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
												<div className="pl-4 border-l border-l-new.primary-25 flex justify-between items-center">
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

			{/* Events Filter Dialog Old*/}
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
												options={editorOptions}
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
												options={editorOptions}
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
												options={editorOptions}
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
												options={editorOptions}
											/>
										</div>
									</div>
								</div>
							</TabsContent>
						</Tabs>
					</div>
				</DialogContent>
			</Dialog>

			{/* Events Filter Dialog New*/}
			<Dialog
				open={showEventsFilterDialog}
				onOpenChange={setShowEventsFilterDialog}
			>
				<DialogContent className="w-[90%] h-[90%] max-w-[90%] max-h-[90%] p-0 overflow-hidden rounded-8px gap-0">
					<div className="flex flex-col h-full">
						{/* Dialog Header */}
						<DialogHeader className="px-12 py-4 border-b border-neutral-4">
							<div className="flex items-center justify-between w-full">
								<DialogTitle className="text-base font-semibold">
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
							</div>
						</DialogHeader>
					</div>
					<div className="flex-1 overflow-auto px-6">
						<div className="min-w-[80vw] mx-auto">
							{/* Tabs for Body and Header*/}
							<Tabs defaultValue="body" className="w-full">
								<TabsList className="w-full p-0 bg-background border-b rounded-none flex items-center justify-center">
									<TabsTrigger
										value="body"
										className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
									>
										<span className="text-sm">Body</span>
									</TabsTrigger>
									<TabsTrigger
										value="header"
										className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
									>
										<span className="text-sm">Header</span>
									</TabsTrigger>
								</TabsList>

								{/* Body Tab */}
								<TabsContent value="body">
									<div className="flex w-full border border-neutral-4 rounded-lg">
										{/* Body Event Payload */}
										<div className="flex flex-col w-1/2 border-r border-neutral-4">
											<div>
												{/* Body Input */}
												<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
													Event Payload
												</div>
												<div>
													<Editor
														height="800px"
														language="json"
														defaultValue={eventsFilter.request.body}
														onChange={body =>
															setEventsFilter(prev => ({
																...prev,
																request: { ...prev.request, body: body || '' },
															}))
														}
														options={editorOptions}
													/>
												</div>
											</div>
										</div>

										{/* Body Schema */}
										<div className="flex flex-col w-1/2 border-r border-neutral-4">
											<div>
												{/* Body Filter Schema */}
												<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
													Filter Schema
												</div>
												<div>
													<Editor
														height="800px"
														language="json"
														defaultValue={eventsFilter.schema.body}
														onChange={body =>
															setEventsFilter(prev => ({
																...prev,
																schema: { ...prev.schema, body: body || '' },
															}))
														}
														options={editorOptions}
													/>
												</div>
											</div>
										</div>
									</div>
								</TabsContent>

								<TabsContent value="header">
								<div className="flex w-full border border-neutral-4 rounded-lg">
										{/* Body Event Payload */}
										<div className="flex flex-col w-1/2 border-r border-neutral-4">
											<div>
												{/* Body Input */}
												<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
													Event Payload
												</div>
												<div>
													<Editor
														height="800px"
														language="json"
														defaultValue={eventsFilter.request.header}
														onChange={header =>
															setEventsFilter(prev => ({
																...prev,
																request: { ...prev.request, header: header || '' },
															}))
														}
														options={editorOptions}
													/>
												</div>
											</div>
										</div>

										{/* Body Schema */}
										<div className="flex flex-col w-1/2 border-r border-neutral-4">
											<div>
												{/* Body Filter Schema */}
												<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
													Filter Schema
												</div>
												<div>
													<Editor
														height="800px"
														language="json"
														defaultValue={eventsFilter.schema.header}
														onChange={header =>
															setEventsFilter(prev => ({
																...prev,
																schema: { ...prev.schema, header: header || '' },
															}))
														}
														options={editorOptions}
													/>
												</div>
											</div>
										</div>
									</div>
								</TabsContent>
							</Tabs>
						</div>
					</div>
				</DialogContent>
			</Dialog>

			{/* Transform Function Dialog */}
			<Dialog
				open={showTransformFunctionDialog}
				onOpenChange={open => {
					if (!open && !hasSavedFn) {
						alert('You have not saved your function'); // TODO use toast instead
						setShowTransformFunctionDialog(false);
						return;
					}
					if (!open) setShowTransformFunctionDialog(false);
				}}
			>
				<DialogContent className="w-[90%] h-[90%] max-w-[90%] max-h-[90%] p-0 overflow-hidden rounded-8px gap-0">
					<div className="flex flex-col h-full">
						{/* Dialog Header */}
						<DialogHeader className="px-12 py-4 border-b border-neutral-4">
							<div className="flex items-center justify-between w-full">
								<DialogTitle className="text-base font-semibold">
									Source Transform
								</DialogTitle>

								<DialogDescription className="sr-only">
									Source Transform
								</DialogDescription>
								<Button
									variant="ghost"
									size={'sm'}
									className="px-4 py-2 bg-new.primary-400 text-white-100 hover:bg-new.primary-400 hover:text-white-100 text-xs"
									onClick={() => {
										setHasSavedFn(true);
										setShowTransformFunctionDialog(false);
									}}
									disabled={!isTransformPassed}
								>
									<Save className="stroke-white-100" />
									Save Function
								</Button>
							</div>
						</DialogHeader>

						{/* Dialog Body */}
						<div className="flex-1 overflow-auto px-6">
							<div className="min-w-[80vw] mx-auto">
								{/* Tabs for Body/Header */}
								<Tabs defaultValue="body" className="w-full">
									<TabsList className="w-full p-0 bg-background border-b rounded-none flex items-center justify-center">
										<TabsTrigger
											value="body"
											className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
										>
											<span className="text-sm">Body</span>
										</TabsTrigger>
										<TabsTrigger
											value="header"
											className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
										>
											<span className="text-sm">Header</span>
										</TabsTrigger>
									</TabsList>
									{/* Body Tab*/}
									<TabsContent value="body">
										<div className="flex w-full border border-neutral-4 rounded-lg">
											<div className="flex flex-col w-1/2 border-r border-neutral-4">
												<div>
													{/* Body Input */}
													<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
														Input
													</div>
													<div className="h-[300px]">
														<Editor
															height="300px"
															language="json"
															value={JSON.stringify(
																transformBodyPayload,
																null,
																2,
															)}
															onChange={value =>
																setTransformBodyPayload(
																	JSON.parse(value || '{}'),
																)
															}
															options={editorOptions}
														/>
													</div>
												</div>
												<div className="min-h-[370px]">
													<Tabs defaultValue="output">
														<TabsList className="w-full p-0 bg-background border-b rounded-none flex items-center justify-start">
															<TabsTrigger
																value="output"
																className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
															>
																<span className="text-xs">Output</span>
															</TabsTrigger>
															<TabsTrigger
																value="diff"
																className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
															>
																<span className="text-xs">Diff</span>
															</TabsTrigger>
														</TabsList>
														<div className="h-[336px]">
															{/* Body Output */}
															<TabsContent value="output" className="h-[336px]">
																<Editor
																	height="336px"
																	language="json"
																	value={
																		typeof bodyOutput.current === 'object'
																			? JSON.stringify(
																					bodyOutput.current,
																					null,
																					2,
																				)
																			: `${bodyOutput.current}`
																	}
																	options={{ ...editorOptions, readOnly: true }}
																/>
															</TabsContent>
															{/* Body Diff */}
															<TabsContent value="diff" className="h-[336px]">
																<DiffEditor
																	height="336px"
																	language="json"
																	original={
																		typeof bodyOutput.previous === 'object'
																			? JSON.stringify(
																					bodyOutput.previous,
																					null,
																					2,
																				)
																			: `${bodyOutput.previous}`
																	}
																	modified={
																		// TODO change to body diff
																		typeof bodyOutput.current === 'object'
																			? JSON.stringify(
																					bodyOutput.current,
																					null,
																					2,
																				)
																			: `${bodyOutput.current}`
																	}
																	options={{ ...editorOptions, readOnly: true }}
																/>
															</TabsContent>
														</div>
													</Tabs>
												</div>
											</div>
											<div className="flex flex-col w-1/2">
												<div className="flex justify-between items-center text-xs  border-b border-neutral-4 px-3 py-2 rounded-tr-lg">
													<span className="text-neutral-11">
														Transform Function
													</span>
													<Button
														variant="outline"
														size="sm"
														className="h-6 py-0 px-2 text-xs border border-new.primary-300 text-new.primary-300 gap-1 hover:text-new.primary-300 hover:bg-white-100 shadow-none"
														disabled={isTestingFunction}
														onClick={() => {
															setShowConsole(true);
															testTransformFunction('body');
														}}
													>
														<svg
															width="10"
															height="11"
															viewBox="0 0 10 11"
															fill="none"
															xmlns="http://www.w3.org/2000/svg"
															className=""
														>
															<path
																d="M1.66797 5.5004V4.01707C1.66797 2.1754 2.97214 1.42124 4.56797 2.34207L5.85547 3.08374L7.14297 3.8254C8.7388 4.74624 8.7388 6.25457 7.14297 7.1754L5.85547 7.91707L4.56797 8.65874C2.97214 9.57957 1.66797 8.8254 1.66797 6.98374V5.5004Z"
																stroke="#477DB3"
																strokeMiterlimit="10"
																strokeLinecap="round"
																strokeLinejoin="round"
															/>
														</svg>
														Run
													</Button>
												</div>
												{/* Body Transform Function */}
												<div
													className={showConsole ? 'h-[500px]' : 'h-[632px]'}
												>
													<Editor
														height="100%"
														language="javascript"
														value={transformFnBody}
														onChange={value => {
															setTransformFnBody(value || '');
															setHasSavedFn(false);
														}}
														options={editorOptions}
													/>
												</div>

												{/* Body Console */}
												{(showConsole || bodyLogs.length > 0) && (
													<div className="border-t border-neutral-4">
														<div className="flex justify-between items-center px-3 py-1.5 text-xs text-neutral-11">
															<span>Console</span>
															<Button
																variant="ghost"
																size="sm"
																className="h-5 w-5 p-0"
																onClick={() => setShowConsole(false)}
															>
																<svg
																	width="14"
																	height="14"
																	className="fill-neutral-10"
																>
																	<use xlinkHref="#close-icon"></use>
																</svg>
															</Button>
														</div>
														<div className="h-[132px] bg-neutral-1 p-2 overflow-auto">
															{bodyLogs.map((log, index) => (
																<div
																	key={index}
																	className="text-xs font-mono whitespace-pre-wrap"
																>
																	{log}
																</div>
															))}
														</div>
													</div>
												)}
											</div>
										</div>
									</TabsContent>

									{/* Header Tab */}
									<TabsContent value="header">
										<div className="flex w-full border border-neutral-4 rounded-lg">
											<div className="flex flex-col w-1/2 border-r border-neutral-4">
												<div>
													<div className="text-xs text-neutral-11 border-b border-neutral-4 p-3 rounded-tl-lg">
														Input
													</div>
													{/* Header Input */}
													<div className="h-[300px]">
														<Editor
															height="300px"
															language="json"
															value={JSON.stringify(headerPayload, null, 2)}
															onChange={value =>
																setHeaderPayload(JSON.parse(value || '{}'))
															}
															options={{ ...editorOptions, readOnly: false }}
														/>
													</div>
												</div>
												<div className="min-h-[370px]">
													<Tabs defaultValue="output">
														<TabsList className="w-full p-0 bg-background border-b rounded-none flex items-center justify-start">
															<TabsTrigger
																value="output"
																className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
															>
																<span className="text-xs">Output</span>
															</TabsTrigger>
															<TabsTrigger
																value="diff"
																className="rounded-none bg-background h-full data-[state=active]:shadow-none border-b-2 border-transparent data-[state=active]:border-new.primary-400"
															>
																<span className="text-xs">Diff</span>
															</TabsTrigger>
														</TabsList>
														<TabsContent value="output">
															<div className="h-[336px]">
																<Editor
																	height="336px"
																	language="json"
																	value={
																		typeof headerOutput.current === 'object'
																			? JSON.stringify(
																					headerOutput.current,
																					null,
																					2,
																				)
																			: String(headerOutput.current)
																	}
																	options={{ ...editorOptions, readOnly: true }}
																/>
															</div>
														</TabsContent>
														<TabsContent value="diff">
															<DiffEditor
																height="336px"
																language="json"
																original={
																	typeof headerOutput.previous === 'object'
																		? JSON.stringify(
																				headerOutput.previous,
																				null,
																				2,
																			)
																		: String(headerOutput.previous)
																}
																modified={
																	typeof headerOutput.current === 'object'
																		? JSON.stringify(
																				headerOutput.current,
																				null,
																				2,
																			)
																		: String(headerOutput.current)
																}
																options={{ ...editorOptions, readOnly: true }}
															/>
														</TabsContent>
													</Tabs>
												</div>
											</div>
											<div className="flex flex-col w-1/2">
												<div className="flex justify-between items-center text-xs text-neutral-11 border-b border-neutral-4 px-3 py-2 rounded-tr-lg">
													<span>Transform Function</span>
													<Button
														variant="outline"
														size="sm"
														disabled={isTestingFunction}
														className="h-6 py-0 px-2 text-xs border border-new.primary-300 text-new.primary-300 gap-1 hover:text-new.primary-300 hover:bg-white-100 shadow-none"
														onClick={() => testTransformFunction('header')}
													>
														<svg
															width="10"
															height="11"
															viewBox="0 0 10 11"
															fill="none"
															xmlns="http://www.w3.org/2000/svg"
															className=""
														>
															<path
																d="M1.66797 5.5004V4.01707C1.66797 2.1754 2.97214 1.42124 4.56797 2.34207L5.85547 3.08374L7.14297 3.8254C8.7388 4.74624 8.7388 6.25457 7.14297 7.1754L5.85547 7.91707L4.56797 8.65874C2.97214 9.57957 1.66797 8.8254 1.66797 6.98374V5.5004Z"
																stroke="#477DB3"
																strokeMiterlimit="10"
																strokeLinecap="round"
																strokeLinejoin="round"
															/>
														</svg>
														Run
													</Button>
												</div>
												{/* Header Transform Function*/}
												<div
													className={showConsole ? 'h-[500px]' : 'h-[632px]'}
												>
													<Editor
														height="100%"
														language="javascript"
														value={transformFnHeader}
														onChange={value => {
															setTransformFnHeader(value || '');
															setHasSavedFn(false);
														}}
														options={editorOptions}
													/>
												</div>

												{showConsole && headerLogs.length > 0 && (
													<div className="border-t border-neutral-4">
														<div className="flex justify-between items-center px-3 py-2 text-xs text-neutral-11">
															<span>Console</span>
															<Button
																variant="ghost"
																size="sm"
																className="h-5 w-5 p-0"
																onClick={() => setShowConsole(false)}
															>
																<svg
																	width="14"
																	height="14"
																	className="fill-neutral-10"
																>
																	<use xlinkHref="#close-icon"></use>
																</svg>
															</Button>
														</div>
														<div className="h-[132px] bg-neutral-1 p-2 overflow-auto">
															{headerLogs.map((log, index) => (
																<div
																	key={index}
																	className="text-xs font-mono whitespace-pre-wrap"
																>
																	{log}
																</div>
															))}
														</div>
													</div>
												)}
											</div>
										</div>
									</TabsContent>
								</Tabs>
							</div>
						</div>
					</div>
				</DialogContent>
			</Dialog>
		</DashboardLayout>
	);
}
