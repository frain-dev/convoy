require('dotenv').config();
const fetch = require('node-fetch');
const { MAILCHIMP_LIST_ID, MAILCHIMP_AUTH } = process.env;

exports.handler = async (event, _context) => {
	if (event.httpMethod !== 'POST') {
		return { statusCode: 405, body: 'Method Not Allowed' };
	}

	const errorGen = msg => {
		return { statusCode: 500, body: msg };
	};

	try {
		const { email } = JSON.parse(event.body);
		if (!email) {
			return errorGen('Form details missing');
		}

		const subscriber = {
			email_address: email,
			status: 'subscribed',
			merge_fields: {
				EMAIL: email,
				PRODUCT: 'Convoy'
			}
		};

		const response = await fetch(`https://us1.api.mailchimp.com/3.0/lists/${MAILCHIMP_LIST_ID}/members/`, {
			method: 'POST',
			headers: {
				Accept: '*/*',
				'Content-Type': 'application/json',
				Authorization: `auth ${MAILCHIMP_AUTH}`
			},
			body: JSON.stringify(subscriber)
		});
		const data = await response.json();

		if (!response.ok) {
			return { statusCode: data.status, body: data.detail };
		}

		return {
			statusCode: 200,
			body: JSON.stringify({
				message: `Welcome on board, your slot has been reserved, 😉`,
				detail: data
			})
		};
	} catch (err) {
		return {
			statusCode: 500,
			body: JSON.stringify({ msg: err.message })
		};
	}
};
