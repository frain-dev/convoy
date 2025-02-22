import axios from 'axios';

type SendWelcomeEmailParams = {
	email: string;
	firstname: string;
	lastname: string;
};

function createWelcomeEmailUrl(args: SendWelcomeEmailParams) {
	const { email, firstname, lastname } = args;
	return `https://faas-fra1-afec6ce7.doserverless.co/api/v1/web/fn-8f44e6aa-e5d6-4e31-b781-5080c050bb37/welcome-user/welcome-mail?email=${email}&firstname=${firstname}&lastname=${lastname}`;
}

export async function sendWelcomeEmail(
	args: SendWelcomeEmailParams,
	deps: { httpGet: typeof axios.get } = { httpGet: axios.create().get },
) {
	try {
		const { data } = await deps.httpGet(createWelcomeEmailUrl(args));

		return data;
	} catch (err) {
		if (axios.isAxiosError(err)) {
			console.log('hubspot error message: ', err.message);
			return err.message;
		}

		console.log('hubspot unexpected error: ', err);
		throw new Error('An hubspot unexpected error occurred');
	}
}
