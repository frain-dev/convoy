import { useState } from 'react';
import { createFileRoute, useNavigate } from '@tanstack/react-router';
import * as signUpService from '@/services/signup.service';
import * as hubSpotService from '@/services/hubspot.service';
import * as licensesService from '@/services/licenses.service';
import { router } from '@/lib/router';
import { CONVOY_DASHBOARD_DOMAIN } from '@/lib/constants';

export const Route = createFileRoute('/signup')({
	async beforeLoad() {
		try {
			await licensesService.setLicenses();
			const hasCreateUserLicense = licensesService.hasLicense('CREATE_USER');
			if (!hasCreateUserLicense)
				throw new Error('beforeLoad: client cannot create user');
		} catch (err) {
			console.error('beforeLoad:', err);
			router.navigate({ to: '/' });
		}
	},
	component: SignUpPage,
});

function SignUpPage() {
	const navigate = useNavigate();
	const [isSignUpButtonEnabled, setIsSignUpButtonEnabled] = useState(true);

	async function signUp(values: any) {
		setIsSignUpButtonEnabled(false);
		const { email, firstName, lastName, orgName, password } = values;

		try {
			await signUpService.signUp({
				email,
				first_name: firstName,
				last_name: lastName,
				org_name: orgName,
				password,
			});

			if (location.hostname == CONVOY_DASHBOARD_DOMAIN) {
				await hubSpotService.sendWelcomeEmail({
					email,
					firstname: firstName,
					lastname: lastName,
				});
			}

			setIsSignUpButtonEnabled(false);
			navigate({
				from: '/signup',
				to: '/get-started',
			});
		} catch (err) {
			// TODO show user error on UI
			setIsSignUpButtonEnabled(false);
		}
	}

	async function signUpWithSAML() {
		localStorage.setItem('AUTH_TYPE', 'signup');

		try {
			const { data } = await signUpService.signUpWithSAML();
			window.open(data.redirectUrl, '_blank');
		} catch (err) {
			// TODO show user on the UI
			throw err;
		}
	}

	return <h2 className="text-4xl font-semibold p-4">Sign Up</h2>;
}
