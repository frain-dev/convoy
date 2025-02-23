import { Button } from '@/components/ui/button';
import {
	FormField,
	FormItem,
	FormLabel,
	FormControl,
	FormMessageWithErrorIcon,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Form } from '@/components/ui/form';
import { cn } from '@/lib/utils';
import { zodResolver } from '@hookform/resolvers/zod';
import { EyeIcon, EyeOffIcon } from 'lucide-react';
import { useEffect, useReducer, useState } from 'react';
import { useForm } from 'react-hook-form';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import { z } from 'zod';
import { ConvoyLoader } from '@/components/ConvoyLoader';
import * as loginService from '@/services/login.service';
import * as signUpService from '@/services/signup.service';
import * as privateService from '@/services/private.service';
import * as licensesService from '@/services/licenses.service';

import type { UseFormReturn } from 'react-hook-form';

const formSchema = z.object({
	email: z.string().email('Please enter your email'),
	password: z.string().min(1, 'Please enter your password'),
});

type EmailInputFieldProps = {
	form: UseFormReturn<z.infer<typeof formSchema>>;
};

function EmailInputField({ form }: EmailInputFieldProps) {
	return (
		<FormField
			control={form.control}
			name="email"
			render={({ field, fieldState }) => (
				<FormItem className="w-full relative mb-6 block">
					<div className="w-full mb-2 flex items-center justify-between">
						<FormLabel className="text-xs/5 text-neutral-9">Email</FormLabel>
					</div>
					<FormControl>
						<Input
							autoComplete="email"
							type="email"
							className={cn(
								'mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
								fieldState.error
									? 'border-new.error-500 focus-visible:ring-0 hover:border-new.error-500'
									: ' hover:border-new.primary-100 focus:border-new.primary-300',
							)}
							placeholder="super@default.com"
							{...field}
						/>
					</FormControl>
					<FormMessageWithErrorIcon />
				</FormItem>
			)}
		/>
	);
}

type PasswordInputFieldProps = {
	form: UseFormReturn<z.infer<typeof formSchema>>;
};

function PasswordInputField({ form }: PasswordInputFieldProps) {
	const [isPasswordVisible, setIsPasswordVisible] = useState(false);

	return (
		<FormField
			control={form.control}
			name="password"
			render={({ field, fieldState }) => (
				<FormItem className="w-full relative mb-2 block">
					<div className="w-full mb-[8px] flex items-center justify-between">
						<FormLabel
							className="text-xs/5 text-neutral-9"
							htmlFor="password_input"
						>
							Password
						</FormLabel>
					</div>
					<FormControl className="w-full relative">
						<div className="relative">
							<Input
								id="password_input"
								autoComplete="current-password"
								type={isPasswordVisible ? 'text' : 'password'}
								className={cn(
									'hide-password-toggle mt-0 outline-none focus-visible:ring-0 border-neutral-4 shadow-none w-full h-auto transition-all duration-300 bg-white-100 py-3 px-4 text-neutral-11 !text-xs/5 rounded-[4px] placeholder:text-new.gray-300 placeholder:text-sm/5 font-normal disabled:text-neutral-6 disabled:border-new.primary-25',
									fieldState.error
										? 'border-new.error-500 focus-visible:ring-0 hover:border-new.error-500'
										: 'hover:border-new.primary-100 focus:border-new.primary-300',
								)}
								placeholder="super@default.com"
								{...field}
							/>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								className="absolute right-[1%] top-0 h-full px-3 py-2 hover:bg-transparent"
								onClick={() => setIsPasswordVisible(!isPasswordVisible)}
							>
								{isPasswordVisible ? (
									<EyeIcon className="opacity-50" aria-hidden="true" />
								) : (
									<EyeOffIcon className="opacity-50" aria-hidden="true" />
								)}
								<span className="sr-only">
									{isPasswordVisible ? 'Hide password' : 'Show password'}
								</span>
							</Button>
						</div>
					</FormControl>
					<FormMessageWithErrorIcon />
				</FormItem>
			)}
		/>
	);
}

function ForgotPasswordSection() {
	const navigate = useNavigate();

	function navigateToPasswordPage() {
		navigate({
			from: '/login',
			to: '/forgot-password',
		});
	}

	return (
		<div className="flex items-center text-xs/5 mb-[20px]">
			Forgot password?
			<Button
				variant={'link'}
				type="button"
				className="p-2 ml-[16px] text-new.primary-400 underline underline-offset-1"
				onClick={navigateToPasswordPage}
				size="sm"
			>
				Reset it here
			</Button>
		</div>
	);
}

function LoginButton(props: { isButtonEnabled?: boolean }) {
	const { isButtonEnabled } = props;

	return (
		<Button
			disabled={!isButtonEnabled}
			variant={'ghost'}
			size={'lg'}
			className="flex items-center justify-center disabled:opacity-50 cursor-pointer mb-3 bg-new.primary-400 hover:bg-new.primary-400 px-9 py-[10px] rounded-8px text-sm/5 text-white-100 w-full"
		>
			<span
				className={cn(
					'text-sm text-white-100',
					!isButtonEnabled ? 'hidden' : '',
				)}
			>
				Login
			</span>
			<img
				className={cn('h-4', !isButtonEnabled ? '' : 'hidden')}
				src="assets/img/button-loader.gif"
				alt="loader"
			/>
		</Button>
	);
}

function LoginWithSAMLButton() {
	async function login() {
		localStorage.setItem('AUTH_TYPE', 'login');

		try {
			const res = await loginService.loginWithSAML();
			const { redirectUrl } = res.data;
			window.open(redirectUrl);
		} catch (error) {
			// TODO should notify user here with UI
			throw error;
		}
	}

	return (
		<Button
			type="button"
			size={'lg'}
			variant={'ghost'}
			onClick={login}
			className="disabled:opacity-50 cursor-pointer w-full rounded-8px text-xs/5 text-new.primary-400 hover:text-new.primary-400 py-0 hover:bg-transparent h-auto"
		>
			Login with SAML
		</Button>
	);
}

function SignUpButton() {
	const navigate = useNavigate();

	function navigateToSignUpPage() {
		navigate({
			from: '/login',
			to: '/signup',
		});
	}

	return (
		<Button
			type="button"
			size={'lg'}
			variant={'ghost'}
			onClick={navigateToSignUpPage}
			className="disabled:opacity-50 cursor-pointer w-full rounded-8px text-xs/5 text-new.primary-400 hover:text-new.primary-400 py-0 hover:bg-transparent h-auto mt-3"
		>
			Sign Up
		</Button>
	);
}

type ReducerPayload = Partial<{
	isSignUpEnabled: boolean;
	isFetchingConfig: boolean;
	isLoadingProject: boolean;
	hasCreateUserLicense: boolean;
	isLoginButtonEnabled: boolean;
}>;

const initialReducerState = {
	isSignUpEnabled: false,
	isFetchingConfig: false,
	isLoadingProject: false,
	isLoginButtonEnabled: true,
	hasCreateUserLicense: false,
};

function reducer(state: ReducerPayload, payload: ReducerPayload) {
	return {
		...state,
		...payload,
	};
}

function LoginPage() {
	const navigate = useNavigate();
	const [state, dispatchState] = useReducer(reducer, initialReducerState);

	useEffect(function () {
		getSignUpConfig();
		licensesService.setLicenses();
		const hasCreateUserLicense = licensesService.hasLicense('CREATE_USER');
		dispatchState({ hasCreateUserLicense });
	}, []);

	const form = useForm<z.infer<typeof formSchema>>({
		resolver: zodResolver(formSchema),
		defaultValues: {
			email: '',
			password: '',
		},
		mode: 'onTouched',
	});

	async function login(values: z.infer<typeof formSchema>) {
		dispatchState({ isLoginButtonEnabled: false });

		try {
			await loginService.login(values);
			dispatchState({ isLoadingProject: true });
			await getOrganisations();
			dispatchState({ isLoginButtonEnabled: true, isLoadingProject: false });

			navigate({
				to: '/',
				from: '/login',
			});
		} catch (err) {
			// TODO notify user using the UI
			console.error(login.name, err);
		}
	}

	async function getSignUpConfig() {
		dispatchState({ isFetchingConfig: true });
		try {
			const { data } = await signUpService.getSignUpConfig();
			dispatchState({ isSignUpEnabled: data });
		} catch (err) {
			// TODO notify user using the UI
			console.error(getSignUpConfig.name, err);
		} finally {
			dispatchState({ isFetchingConfig: false });
		}
	}

	async function getOrganisations() {
		try {
			await privateService.getOrganisations({ refresh: true });
		} catch (err) {
			console.error(getOrganisations.name, err);
		}
	}

	return (
		<>
			<div className="flex w-full">
				<aside className="bg-primary-100 bg-[url('/assets/img/public-layout.png')] bg-no-repeat bg-right-top min-w-[374px] desktop:w-0 h-screen transition-all duration-300 px-[36px] pt-[70px]"></aside>
				<div className="bg-[url('/assets/svg/pattern.svg')] bg-center bg-cover min-h-screen w-full">
					<div className="min-h-screen flex flex-col items-center justify-center w-full">
						<img
							src="/assets/svg/logo.svg"
							alt="convoy logo"
							className="mb-7 w-[130px]"
						/>

						<section className="max-w-[445px] mx-auto my-0 p-6 w-full bg-white-100 shadow-default rounded-[8px]">
							<Form {...form}>
								<form
									onSubmit={(...args) => void form.handleSubmit(login)(...args)}
								>
									<EmailInputField form={form} />

									<PasswordInputField form={form} />

									<ForgotPasswordSection />

									<LoginButton isButtonEnabled={state.isLoginButtonEnabled} />

									<LoginWithSAMLButton />
								</form>
							</Form>

							{state.isSignUpEnabled && state.hasCreateUserLicense && (
								<SignUpButton />
							)}
						</section>
					</div>
				</div>
			</div>

			<ConvoyLoader
				isTransparent={false}
				isVisible={state.isLoadingProject || state.isFetchingConfig}
			/>
		</>
	);
}

export const Route = createFileRoute('/login')({
	component: LoginPage,
});

// TODO loginService and other impure extraneous deps should be injected as a
// dependency for testing and flexibility/maintainability
