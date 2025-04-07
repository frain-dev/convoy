import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useState, useCallback, useMemo } from 'react';
import { Editor, DiffEditor } from '@monaco-editor/react';
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router';

import { Check, ChevronDown, Info, Save, ChevronRight, X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
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
import { Command as CommandPrimitive } from 'cmdk';

import { cn } from '@/lib/utils';
import { stringToJson } from '@/lib/pipes';
import * as authService from '@/services/auth.service';
import { useLicenseStore, useProjectStore } from '@/store';
import * as sourcesService from '@/services/sources.service';
import * as projectsService from '@/services/projects.service';
import * as endpointsService from '@/services/endpoints.service';
import * as subscriptionsService from '@/services/subscriptions.service';

import type { KeyboardEvent } from 'react';

import githubIcon from '../../../../../assets/img/github.png';
import shopifyIcon from '../../../../../assets/img/shopify.png';
import twitterIcon from '../../../../../assets/img/twitter.png';
import modalCloseIcon from '../../../../../assets/svg/modal-close-icon.svg';

export const Route = createFileRoute(
	'/projects_/$projectId/subscriptions/$subscriptionId',
)({
	component: UpdateSubscriptionPage,
	async loader({ params }) {
		const subscription = await subscriptionsService.getSubscription(
			params.subscriptionId,
		);
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
		const { event_types } = await projectsService.getEventTypes(
			params.projectId,
		);

		return {
			subscription,
			canManageSubscriptions: perms.includes('Subscriptions|MANAGE'),
			existingSources: sources.content,
			existingEndpoints: endpoints.data.content,
			hasAdvancedEndpointManagement,
			hasAdvancedSubscriptions,
			hasWebhookTransformations,
			eventTypes: event_types
				.filter(et => et.deprecated_at === null)
				.map(({ name }) => name),
		};
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

type SourceType = (typeof sourceVerifications)[number]['uid'];

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

const SourceFormSchema = z.object({
	name: z.string(),
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
	}),
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
});

const EndpointFormSchema = z
	.object({
		name: z.string(),
		url: z.union([z.literal(''), z.string().trim().url()]),
		secret: z.string().optional(),
		owner_id: z.string().optional(),
		http_timeout: z
			.string()
			.optional()
			.transform(v => (v === '' ? undefined : Number(v))),
		rate_limit: z
			.string()
			.optional()
			.transform(v => (v === '' ? undefined : Number(v))),
		rate_limit_duration: z
			.string()
			.optional()
			.transform(v => (v === '' ? undefined : Number(v))),
		support_email: z.union([z.literal(''), z.string().trim().email()]),
		slack_webhook_url: z.union([z.literal(''), z.string().trim().url()]),
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
		advanced_signatures: z.enum(['true', 'false']).transform(v => v === 'true'),
		showHttpTimeout: z.boolean(),
		showRateLimit: z.boolean(),
		showOwnerId: z.boolean(),
		showAuth: z.boolean(),
		showNotifications: z.boolean(),
		showSignatureFormat: z.boolean(),
	})
	.transform(v => {
		if (!v.showAuth) {
			v.authentication = undefined;
			return v;
		}
		return v;
	});

const SubscriptionFormSchema = z
	.object({
		name: z.string().min(1, 'Enter new subscription name'),
		projectType: z.enum(['incoming', 'outgoing']),
		function: z
			.string()
			.nullable()
			.transform(v => (v?.length ? v : null)), // transform function
		filter_config: z.object({
			event_types: z
				.array(z.string())
				.nullable()
				.transform(v => (v?.length ? v : null))
				.transform(v => (v?.length ? v : null)),
			filter: z.object({
				headers: z.any().transform(v => {
					const isEmptyObj =
						typeof v === 'object' && v !== null && Object.keys(v).length == 0;
					if (isEmptyObj) return null;
					return v;
				}),
				body: z.any().transform(v => {
					const isEmptyObj =
						typeof v === 'object' && v !== null && Object.keys(v).length == 0;
					if (isEmptyObj) return null;
					return v;
				}),
			}),
		}),
		source_id: z.string({ required_error: 'Select source' }).optional(),
		source: SourceFormSchema.nullable(),
		endpoint_id: z.string().optional(),
		endpoint: EndpointFormSchema.nullable(),
		showEventsFilter: z.boolean(),
		showEventTypes: z.boolean(),
		showTransform: z.boolean(),
		useExistingEndpoint: z.boolean(),
		useExistingSource: z.boolean(),
	})
	.refine(
		({ showEventTypes, filter_config }) => {
			if (!showEventTypes) return true;

			return filter_config?.event_types?.length;
		},
		{
			message: 'Include at least one event type when event types is enabled',
			path: ['filter_config.event_types'],
		},
	)
	.refine(({ showTransform, function: fn }) => !(showTransform && !fn), {
		message: 'Set transform function when Transform is enabled',
		path: ['function'],
	})
	.refine(
		({ useExistingEndpoint, endpoint_id }) =>
			!(useExistingEndpoint && !endpoint_id),
		{ message: 'Select endpoint', path: ['endpoint_id'] },
	)
	.refine(
		({ useExistingSource, source_id, projectType }) =>
			!(projectType == 'incoming' && useExistingSource && !source_id),
		{ message: 'Select source', path: ['source_id'] },
	)
	.refine(
		({ useExistingSource, source, projectType }) => {
			if (useExistingSource || projectType == 'outgoing') return true;
			if (
				source?.showCustomResponse &&
				(!source?.custom_response?.content_type ||
					!source?.custom_response.body)
			) {
				return false;
			}

			return true;
		},
		({ source }) => {
			if (!source?.custom_response?.content_type)
				return {
					message: 'Enter content type',
					path: ['source.custom_response.content_type'],
				};

			if (!source?.custom_response?.body)
				return {
					message: 'Enter response content',
					path: ['source.custom_response.body'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ source, useExistingSource, projectType }) => {
			if (useExistingSource || projectType == 'outgoing') return true;

			if (source?.showIdempotency && source?.idempotency_keys?.length == 0)
				return false;
			return true;
		},
		() => {
			return {
				message:
					'Add at least one idempotency key if idempotency configuration is enabled',
				path: ['source.idempotency_keys'],
			};
		},
	)
	.refine(
		({ source, useExistingSource, projectType }) => {
			if (useExistingSource || projectType == 'outgoing') return true;
			if (!source) return false;

			const { encoding, hash, header, secret } = source.config;
			const hasInvalidValue = !encoding || !hash || !header || !secret;
			if (source.type == 'hmac' && hasInvalidValue) return false;

			return true;
		},
		({ source }) => {
			if (!source) return { message: '', path: [] };

			const { encoding, hash, header, secret } = source.config;
			if (!encoding)
				return {
					message: 'Enter encoding value',
					path: ['source.config.encoding'],
				};

			if (!hash)
				return {
					message: 'Enter hash value',
					path: ['source.config.hash'],
				};

			if (!header)
				return {
					message: 'Enter header key',
					path: ['source.config.header'],
				};

			if (!secret)
				return {
					message: 'Enter webhook signing secret',
					path: ['source.config.secret'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ useExistingSource, source, projectType }) => {
			if (useExistingSource || projectType == 'outgoing') return true;
			if (!source) return false;

			const { secret } = source.config;
			const isPreconfigured = ['github', 'shopify', 'twitter'].includes(
				source?.type,
			);
			if (isPreconfigured && !secret) return false;
			return true;
		},
		() => ({
			message: 'Enter webhook signing secret',
			path: ['source.config.secret'],
		}),
	)
	.refine(
		({ source, useExistingSource, projectType }) => {
			if (useExistingSource || projectType == 'outgoing') return true;
			if (!source) return false;

			const { username, password } = source.config;
			const hasInvalidValue = !username || !password;
			if (source.type == 'basic_auth' && hasInvalidValue) return false;
			return true;
		},
		({ source }) => {
			if (!source) return { message: '', path: [] };

			const { username, password } = source.config;
			if (!username)
				return {
					message: 'Enter username',
					path: ['source.config.username'],
				};

			if (!password)
				return {
					message: 'Enter password',
					path: ['source.config.password'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ useExistingSource, source, projectType }) => {
			if (useExistingSource || projectType == 'outgoing') return true;
			if (!source) return false;

			const { header_name, header_value } = source.config;
			const hasInvalidValue = !header_name || !header_value;
			if (source.type == 'api_key' && hasInvalidValue) return false;

			return true;
		},
		({ source }) => {
			if (!source) return { message: '', path: [] };

			const { header_name, header_value } = source.config;
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
	)
	.refine(
		({ useExistingSource, source, projectType }) => {
			if (useExistingSource || projectType == 'outgoing') return true;
			if (!source) return false;

			if (!source.name) return false;

			return true;
		},
		{
			message: 'Enter source name',
			path: ['source.name'],
		},
	)
	.refine(
		({ useExistingEndpoint, endpoint }) => {
			if (useExistingEndpoint) return true;
			if (!endpoint?.name || !endpoint?.url) return false;
			return true;
		},
		({ endpoint }) => {
			if (!endpoint?.name)
				return {
					message: 'Enter endpoint name',
					path: ['endpoint.name'],
				};

			if (!endpoint?.url)
				return {
					message: 'Enter endpoint URL',
					path: ['endpoint.url'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ useExistingEndpoint, endpoint }) => {
			if (useExistingEndpoint) return true;

			if (endpoint?.showHttpTimeout && !endpoint.http_timeout) return false;

			return true;
		},
		{
			message: 'Timeout is required when enabled',
			path: ['endpoint.http_timeout'],
		},
	)
	.refine(
		({ useExistingEndpoint, endpoint }) => {
			if (useExistingEndpoint) return true;

			if (endpoint?.showRateLimit && !endpoint.rate_limit) return false;

			if (endpoint?.showRateLimit && !endpoint.rate_limit_duration)
				return false;

			return true;
		},
		({ endpoint }) => {
			if (!endpoint?.rate_limit)
				return {
					message: 'Rate limit is required when enabled',
					path: ['endpoint.rate_limit'],
				};

			if (!endpoint?.rate_limit)
				return {
					message: 'Rate limit duration is required when enabled',
					path: ['endpoint.rate_limit_duration'],
				};

			return {
				message: '',
				path: [],
			};
		},
	)
	.refine(
		({ useExistingEndpoint, endpoint }) => {
			if (useExistingEndpoint) return true;

			if (endpoint?.showOwnerId && !endpoint.owner_id) return false;

			return true;
		},
		{
			message: 'Owner ID is required when enabled',
			path: ['endpoint.ownerId'],
		},
	)
	.refine(
		({ useExistingEndpoint, endpoint }) => {
			if (useExistingEndpoint) return true;

			if (endpoint?.showAuth && !endpoint.authentication?.api_key?.header_name)
				return false;

			if (endpoint?.showAuth && !endpoint.authentication?.api_key?.header_value)
				return false;

			return true;
		},
		({ endpoint }) => {
			if (!endpoint?.authentication?.api_key?.header_name)
				return {
					message: 'API key is required when auth is enabled',
					path: ['endpoint.authentication.api_key.header_name'],
				};

			if (!endpoint?.authentication?.api_key?.header_name)
				return {
					message: 'API key value is required when auth is enabled',
					path: ['endpoint.authentication.api_key.header_value'],
				};

			return { message: '', path: [] };
		},
	)
	.refine(
		({ useExistingEndpoint, endpoint }) => {
			if (useExistingEndpoint) return true;

			if (
				endpoint?.showNotifications &&
				!endpoint.support_email &&
				!endpoint.slack_webhook_url
			)
				return false;

			return true;
		},
		{
			message: 'One of email or webhook URL is required',
			path: ['endpoint.support_email'],
		},
	)
	.refine(
		({ useExistingEndpoint, endpoint }) => {
			if (useExistingEndpoint) return true;

			if (endpoint?.showSignatureFormat && !endpoint.advanced_signatures)
				return false;
			return true;
		},
		{
			message: 'Signature format is required when enabled',
			path: ['endpoint.advanced_signatures'],
		},
	);

const defaultTransformFn = `/*  1. While you can write multiple functions, the main
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
}`;

const defaultTransformFnHeader = `/* 1. While you can write multiple functions, the main function
called for your transformation is the transform function.

2. The only argument acceptable in the transform function is
the payload data.

3. The transform method must return a value.

4. Console logs lust be written like this
console.log('%j', logged_item) to get printed in the log below. */

function transform(payload) {
// Transform function here
return payload;
}`;

function UpdateSubscriptionPage() {
	const { project } = useProjectStore();
	const { projectId } = Route.useParams();
	const navigate = useNavigate();
	const {
		subscription,
		canManageSubscriptions,
		existingSources,
		existingEndpoints,
		hasAdvancedEndpointManagement,
		hasAdvancedSubscriptions,
		hasWebhookTransformations,
		eventTypes,
	} = Route.useLoaderData();
	const [isCreating, setIsCreating] = useState(false);

	const [isMultiSelectOpen, setIsMultiSelectOpen] = useState(false);
	const [selectedEventTypes, setSelectedEventTypes] = useState<string[]>(
		subscription.filter_config.event_types,
	);

	const [inputValue, setInputValue] = useState('');

	const handleUnselect = useCallback((eventType: string) => {
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
		() => eventTypes.filter(et => !selectedEventTypes.includes(et)),
		[selectedEventTypes, eventTypes],
	);

	const [showEventsFilterDialog, setShowEventsFilterDialog] = useState(false);
	const [hasPassedTestFilter, setHasPassedTestFilter] = useState(false);
	const [eventsFilter, setEventsFilter] = useState({
		request: {
			body: defaultFilterRequestBody,
			header: '',
		},
		// TODO ensure this functions here are passed properly
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
		subscription.source_metadata.body_function || defaultTransformFn,
	);
	const [transformFnHeader, setTransformFnHeader] = useState<string>(
		subscription.source_metadata.header_function || defaultTransformFnHeader,
	);
	const [bodyOutput, setBodyOutput] = useState<FuncOutput>(defaultOutput);
	const [headerOutput, setHeaderOutput] = useState<FuncOutput>(defaultOutput);
	const [bodyLogs, setBodyLogs] = useState<string[]>([]);
	const [headerLogs, setHeaderLogs] = useState<string[]>([]);
	const [transformFn, setTransformFn] = useState<string>(subscription.function);
	// const [headerTransformFn, setHeaderTransformFn] = useState<string>();
	const [hasSavedFn, setHasSavedFn] = useState(false);
	const form = useForm<z.infer<typeof SubscriptionFormSchema>>({
		resolver: zodResolver(SubscriptionFormSchema),
		defaultValues: {
			name: subscription.name,
			projectType: project?.type,
			function: subscription.function,
			filter_config: {
				filter: {
					body: subscription.filter_config.filter.body,
					headers: subscription.filter_config.filter.headers,
				},
				event_types: subscription.filter_config.event_types || [],
			},
			source_id:
				project?.type == 'incoming'
					? subscription.source_metadata?.uid || ''
					: '',
			source: {
				name: subscription.source_metadata.name,
				type:
					subscription.source_metadata.provider ||
					subscription.source_metadata.verifier.type ||
					'noop',
				is_disabled: true,
				config: {
					hash: subscription.source_metadata.verifier.hmac.hash,
					encoding: subscription.source_metadata.verifier.hmac.encoding,
					header: subscription.source_metadata.verifier.hmac.header,
					secret: subscription.source_metadata.verifier.hmac.secret,
					username: subscription.source_metadata.verifier.basic_auth.username,
					password: subscription.source_metadata.verifier.basic_auth.password,
					header_name:
						subscription.source_metadata.verifier.api_key.header_name,
					header_value:
						subscription.source_metadata.verifier.api_key.header_value,
				},
				custom_response: {
					content_type:
						subscription.source_metadata.custom_response.content_type,
					body: subscription.source_metadata.custom_response.body,
				},
				idempotency_keys: subscription.source_metadata.idempotency_keys
					? subscription.source_metadata.idempotency_keys
					: [],
				showHmac: subscription.source_metadata.verifier.type == 'hmac',
				showBasicAuth:
					subscription.source_metadata.verifier.type == 'basic_auth',
				showAPIKey: subscription.source_metadata.verifier.type == 'api_key',
				showGithub: subscription.source_metadata.provider == 'github',
				showTwitter: subscription.source_metadata.provider == 'twitter',
				showShopify: subscription.source_metadata.provider == 'shopify',
				showCustomResponse: subscription.source_metadata.custom_response
					.content_type
					? true
					: false,
				showIdempotency: subscription.source_metadata.idempotency_keys?.length
					? true
					: false,
			},
			endpoint_id: subscription.endpoint_metadata?.uid,
			endpoint: {
				name: subscription.endpoint_metadata?.name,
				url: subscription.endpoint_metadata?.url,
				support_email: subscription.endpoint_metadata?.support_email || '',
				slack_webhook_url:
					subscription.endpoint_metadata?.slack_webhook_url || '',
				secret:
					subscription.endpoint_metadata?.secrets?.at(
						subscription.endpoint_metadata?.secrets.length - 1,
					)?.value ?? '',
				// @ts-expect-error the transform fixes this
				http_timeout: subscription.endpoint_metadata?.http_timeout
					? `${subscription.endpoint_metadata?.http_timeout}`
					: '',
				owner_id: subscription.endpoint_metadata?.owner_id || '',
				// @ts-expect-error the transform fixes this
				rate_limit: `${subscription.endpoint_metadata?.rate_limit}`,
				// @ts-expect-error the transform fixes this
				rate_limit_duration: subscription.endpoint_metadata?.rate_limit_duration
					? `${subscription.endpoint_metadata?.rate_limit_duration}`
					: '',
				authentication: {
					type: 'api_key',
					api_key: {
						header_name:
							subscription.endpoint_metadata?.authentication?.api_key
								?.header_name ?? '',
						header_value:
							subscription.endpoint_metadata?.authentication?.api_key
								?.header_value ?? '',
					},
				},
				// @ts-expect-errorthe transform fixes this
				advanced_signatures: subscription.endpoint_metadata?.advanced_signatures
					? 'true'
					: 'false',
				showHttpTimeout: !!subscription.endpoint_metadata?.http_timeout,
				showRateLimit:
					!!subscription.endpoint_metadata?.rate_limit ||
					!!subscription.endpoint_metadata?.rate_limit_duration,
				showOwnerId: !!subscription.endpoint_metadata?.owner_id,
				showAuth: !!subscription.endpoint_metadata?.rate_limit,
				showNotifications:
					!!subscription.endpoint_metadata?.authentication?.api_key.header_name,
				showSignatureFormat:
					!!subscription.endpoint_metadata?.advanced_signatures,
			},
			showTransform: false,
			showEventTypes: !!subscription.filter_config.event_types.length,
			showEventsFilter: hasAdvancedSubscriptions,
			useExistingEndpoint: !!subscription.endpoint_metadata?.uid,
			useExistingSource:
				project?.type == 'outgoing'
					? true
					: subscription.source_metadata.uid !== '',
		},
		mode: 'onTouched',
	});

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
			form.setValue('filter_config.filter.body', eventFilter.schema.body);
			form.setValue('filter_config.filter.headers', eventFilter.schema.header);
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
					response.log.toReversed().length
						? response.log.toReversed()
						: ['Transform function executed successfully'],
				);
			} else {
				setHeaderOutput(prev => ({
					current: response.payload,
					previous: prev.current,
				}));
				setHeaderLogs(
					response.log.toReversed().length
						? response.log.toReversed()
						: ['Transform function executed successfully'],
				);
			}

			setIsTransformPassed(true);
			setIsTestingFunction(false);
			setShowConsole(bodyLogs.length || headerLogs.length ? true : false);

			if (type === 'body') {
				setTransformFn(transformFunc);
			}

			// else {
			// 	setHeaderTransformFn(transformFunc);
			// 	console.log({ transformFunc });
			// }
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

	function saveTransformFn() {
		form.setValue('function', transformFn);
		form.trigger('function');
	}

	async function setSubscriptionFilter() {
		const eventFilter = await testFilter();
		if (hasPassedTestFilter && eventFilter) {
			const { schema } = eventFilter;
			form.setValue('filter_config.filter.body', schema.body);
			form.setValue('filter_config.filter.headers', schema.header);
		}
	}

	function getVerifierType(
		type: SourceType,
		config: z.infer<typeof SourceFormSchema>['config'],
	) {
		const obj: Record<string, string> = {};

		if (type == 'hmac') {
			return {
				type: 'hmac',
				hmac: Object.entries(config).reduce((acc, record: [string, string]) => {
					const [key, val] = record;
					if (['encoding', 'hash', 'header', 'secret'].includes(key)) {
						acc[key] = val;
						return acc;
					}
					return acc;
				}, obj),
			};
		}

		if (type == 'basic_auth') {
			return {
				type: 'basic_auth',
				basic_auth: Object.entries(config).reduce(
					(acc, record: [string, string]) => {
						const [key, val] = record;
						if (['password', 'username'].includes(key)) {
							acc[key] = val;
							return acc;
						}
						return acc;
					},
					obj,
				),
			};
		}

		if (type == 'api_key') {
			return {
				type: 'api_key',
				api_key: Object.entries(config).reduce(
					// @ts-expect-error types match
					(acc, record: [string, string]) => {
						const [key, val] = record;
						if (['header_name', 'header_value'].includes(key)) {
							return (acc[key] = val);
						}
						return acc;
					},
					obj,
				),
			};
		}

		if (['github', 'shopify', 'twitter'].includes(type)) {
			return {
				type: 'hmac',
				hmac: {
					encoding: type == 'github' ? 'hex' : 'base64',
					hash: 'SHA256',
					header: `X-${type == 'github' ? 'Hub' : type == 'shopify' ? 'Shopify-Hmac' : 'Twitter-Webhooks'}-Signature-256`,
					secret: config.secret,
				},
			};
		}

		return {
			type: 'noop',
			noop: obj,
		};
	}

	// TODO move this to utils/pipes
	function transformIncomingSource(v: z.infer<typeof SourceFormSchema>) {
		return {
			name: v.name,
			is_disabled: v.is_disabled,
			type: 'http',
			custom_response: {
				body: v.custom_response?.body || '',
				content_type: v.custom_response?.content_type || '',
			},
			idempotency_keys: v.idempotency_keys?.length ? v.idempotency_keys : null,
			verifier: getVerifierType(v.type, v.config),
			provider: ['github', 'twitter', 'shopify'].includes(v.type) ? v.type : '',
		};
	}

	async function updateSubscription(
		payload: z.infer<typeof SubscriptionFormSchema>,
	) {
		setIsCreating(true);
		if (!payload.useExistingEndpoint && payload.endpoint?.name) {
			try {
				const res = await endpointsService.addEndpoint(payload.endpoint);
				payload.endpoint_id = res.data.uid;
			} catch (error) {
				console.error('Unable to create new endpoint:', error);
				setIsCreating(false);
				throw new Error('Unable to create new endpoint');
			}
		}

		if (
			!payload.useExistingSource &&
			payload.source?.name &&
			project?.type == 'incoming'
		) {
			try {
				const transformed = transformIncomingSource(payload.source);
				const res = await sourcesService.createSource(transformed);
				payload.source_id = res.uid;
			} catch (error) {
				console.error('Unable to create new source:', error);
				setIsCreating(false);
				throw new Error('Unable to create new source');
			}
		}

		try {
			await subscriptionsService.updateSubscription(subscription.uid, {
				name: payload.name,
				endpoint_id: payload.endpoint_id!,
				source_id: payload.source_id!,
				function: payload.function,
				filter_config: {
					event_types: payload.filter_config.event_types,
					filter: payload.filter_config.filter,
				},
			});

			return navigate({
				to: '/projects/$projectId/subscriptions',
				params: { projectId },
			});
		} catch (error) {
			// TODO notify UI
			console.error('Unable to create subscription');
			console.error(error);
		} finally {
			setIsCreating(false);
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
							Update Subscription
						</h1>
					</div>

					<Form {...form}>
						<form
							className="flex flex-col gap-y-8"
							onSubmit={form.handleSubmit(updateSubscription)}
						>
							{/* Source */}
							{project?.type == 'incoming' && (
								<section>
									<h2 className="font-semibold text-sm">Source</h2>
									<p className="text-xs text-neutral-10 mt-1.5">
										Incoming event source this subscription is connected to.
									</p>

									<div className="border border-neutral-4 p-6 rounded-8px mt-6">
										{form.watch('useExistingSource') ? (
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
																							onSelect={() =>
																								field.onChange(source.uid)
																							}
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
													<FormField
														name="useExistingSource"
														control={form.control}
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<Button
																		disabled={!canManageSubscriptions}
																		variant="ghost"
																		size="sm"
																		type="button"
																		className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
																		onClick={() => field.onChange(!field.value)}
																	>
																		Create New Source
																	</Button>
																</FormControl>
															</FormItem>
														)}
													/>
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
														name="source.showCustomResponse"
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
														name="source.showIdempotency"
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
												{form.watch('source.showCustomResponse') && (
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
												{form.watch('source.showIdempotency') && (
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
													<FormField
														name="useExistingSource"
														control={form.control}
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<Button
																		disabled={!canManageSubscriptions}
																		variant="ghost"
																		type="button"
																		size="sm"
																		className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
																		onClick={() => field.onChange(!field.value)}
																	>
																		Use Existing Source
																	</Button>
																</FormControl>
															</FormItem>
														)}
													/>
												</div>
												{/* <div>
													<Button
														type="button"
														variant="ghost"
														size="sm"
														className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
														onClick={toggleUseExistingSource}
													>
														Use Existing Source
													</Button>
												</div> */}
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
									{form.watch('useExistingEndpoint') ? (
										<div className="space-y-4">
											<FormField
												control={form.control}
												name="endpoint_id"
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
																						onSelect={() =>
																							field.onChange(ep.uid)
																						}
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
											<div>
												<FormField
													name="useExistingEndpoint"
													control={form.control}
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<Button
																	disabled={!canManageSubscriptions}
																	variant="ghost"
																	type="button"
																	size="sm"
																	className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
																	onClick={() => field.onChange(!field.value)}
																>
																	Create New Endpoint
																</Button>
															</FormControl>
														</FormItem>
													)}
												/>
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
												{/* TODO add popover to show if business and disabled */}
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
													<FormField
														control={form.control}
														name="endpoint.showSignatureFormat"
														render={({ field }) => (
															<FormItem>
																<FormControl>
																	<ConvoyCheckbox
																		label="Signature Format"
																		isChecked={field.value}
																		onChange={field.onChange}
																	/>
																</FormControl>
															</FormItem>
														)}
													/>
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
																					className="w-full"
																					key={label}
																				>
																					<label
																						className={cn(
																							'cursor-pointer border border-primary-100 flex items-start gap-x-2 p-4 rounded-sm',
																							// @ts-expect-error the transformation takes care of this
																							field.value == value
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
												<FormField
													name="useExistingEndpoint"
													control={form.control}
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<Button
																	disabled={!canManageSubscriptions}
																	variant="ghost"
																	type="button"
																	size="sm"
																	className="pl-0 bg-white-100 text-new.primary-400 hover:bg-white-100 hover:text-new.primary-400 text-xs"
																	onClick={() => field.onChange(!field.value)}
																>
																	Use Existing Endpoint
																</Button>
															</FormControl>
														</FormItem>
													)}
												/>
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
														/>
													</FormControl>
													<FormMessageWithErrorIcon />
												</FormItem>
											)}
										/>

										<hr />

										<div className="flex gap-x-4 items-center">
											<FormField
												control={form.control}
												name="showEventsFilter"
												render={({ field }) => (
													<FormItem>
														<FormControl>
															<ConvoyCheckbox
																disabled={!hasAdvancedSubscriptions}
																label="Events Filter"
																isChecked={field.value}
																onChange={field.onChange}
															/>
														</FormControl>
													</FormItem>
												)}
											/>

											{project?.type == 'outgoing' && (
												<FormField
													control={form.control}
													name="showEventTypes"
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	disabled={!hasAdvancedSubscriptions}
																	label="Event Types"
																	isChecked={field.value}
																	onChange={field.onChange}
																/>
															</FormControl>
														</FormItem>
													)}
												/>
											)}

											{project?.type == 'incoming' && (
												<FormField
													control={form.control}
													name="showTransform"
													render={({ field }) => (
														<FormItem>
															<FormControl>
																<ConvoyCheckbox
																	disabled={!hasWebhookTransformations}
																	label="Transform"
																	isChecked={field.value}
																	onChange={field.onChange}
																/>
															</FormControl>
														</FormItem>
													)}
												/>
											)}
										</div>

										{form.watch('showEventTypes') && (
											<div className="pl-4 border-l border-l-new.primary-25 flex justify-between items-center">
												<FormField
													control={form.control}
													name="filter_config.event_types"
													render={({ field }) => (
														<FormItem className="w-full flex flex-col gap-y-2">
															<div className="w-full space-y-2 flex items-center">
																<FormLabel className="flex items-center gap-x-2">
																	<span className="text-neutral-9 text-xs/5 ">
																		Event Types
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
																				These are the specifications for the
																				retry mechanism for your endpoints under
																				this subscription. In Linear time retry,
																				event retries are done in linear time,
																				while in Exponential back off retry,
																				events are retried progressively
																				increasing the time before the next
																				retry attempt.
																			</p>
																		</TooltipContent>
																	</Tooltip>
																</FormLabel>
															</div>
															<Command className="overflow-visible">
																<div className="rounded-md border border-input px-3 py-2 text-sm focus-within:ring-0 h-12">
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
																					field.onChange(
																						selectedEventTypes.slice(0, -1),
																					);
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
																		{isMultiSelectOpen &&
																			!!filteredEventTypes.length && (
																				<div className="absolute top-0 z-10 w-full rounded-md border bg-popover text-popover-foreground shadow-md outline-none">
																					<CommandGroup className="h-full overflow-auto">
																						{filteredEventTypes.map(
																							eventType => {
																								return (
																									<CommandItem
																										key={eventType}
																										onMouseDown={e => {
																											e.preventDefault();
																										}}
																										onSelect={() => {
																											setInputValue('');
																											setSelectedEventTypes(
																												prev => {
																													field.onChange([
																														...prev,
																														eventType,
																													]);
																													return [
																														...prev,
																														eventType,
																													];
																												},
																											);
																										}}
																										className={'cursor-pointer'}
																									>
																										{eventType}
																									</CommandItem>
																								);
																							},
																						)}
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

							{/* Submit Button */}
							<div className="flex justify-end w-full">
								<Button
									type="submit"
									disabled={isCreating || !form.formState.isValid}
									variant="ghost"
									className="hover:bg-new.primary-400 text-white-100 text-xs hover:text-white-100 bg-new.primary-400"
								>
									{isCreating ? 'Updating...' : 'Update'} Subscription
									<ChevronRight className="stroke-white-100" />
								</Button>
							</div>
						</form>
					</Form>
				</div>
			</div>

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
																request: {
																	...prev.request,
																	header: header || '',
																},
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
																schema: {
																	...prev.schema,
																	header: header || '',
																},
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
										saveTransformFn();
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
