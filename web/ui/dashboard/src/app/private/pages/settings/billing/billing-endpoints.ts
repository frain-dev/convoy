import {BillingStrategy} from 'src/app/models/billing.model';

export type BillingResource =
	| 'subscription'
	| 'payment_methods'
	| 'organisation'
	| 'address'
	| 'tax_id'
	| 'usage'
	| 'invoices';

/**
 * Single owner of the `licensed_self_hosted` vs org-scoped billing URL branch.
 * Self-hosted resolves to the instance `sh_*` routes; every other strategy uses
 * the org-scoped `/billing/organisations/{orgId}/*` routes. Sub-resources (e.g.
 * `payment_methods/{id}/default`) append to the value returned here so both
 * strategies keep their exact prior paths.
 */
export class BillingEndpoints {
	static billingUrl(strategy: BillingStrategy, resource: BillingResource, orgId: string): string {
		const selfHosted = strategy === 'licensed_self_hosted';
		switch (resource) {
			case 'subscription':
				return selfHosted ? '/billing/sh_subscription' : `/billing/organisations/${orgId}/subscription`;
			case 'payment_methods':
				return selfHosted ? '/billing/sh_payment_methods' : `/billing/organisations/${orgId}/payment_methods`;
			case 'organisation':
				return selfHosted ? '/billing/sh_organisation' : `/billing/organisations/${orgId}`;
			case 'address':
				return selfHosted ? '/billing/sh_address' : `/billing/organisations/${orgId}/address`;
			case 'tax_id':
				return selfHosted ? '/billing/sh_tax_id' : `/billing/organisations/${orgId}/tax_id`;
			case 'usage':
				return selfHosted ? `/billing/sh_usage?orgID=${orgId}` : `/billing/organisations/${orgId}/usage`;
			case 'invoices':
				return selfHosted ? '/billing/sh_invoices' : `/billing/organisations/${orgId}/invoices`;
		}
	}
}
