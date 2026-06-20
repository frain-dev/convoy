import {Injectable} from '@angular/core';
import {HttpService} from '../http/http.service';
import {ConfigService} from '../config/config.service';
import {BillingStrategy} from 'src/app/models/billing.model';

@Injectable({ providedIn: 'root' })
export class OrganisationStateService {
	constructor(private httpService: HttpService, private configService: ConfigService) {}

	// True when the current org has been disabled (e.g. unpaid). Fail-safe: any
	// missing or unparseable org data reads as not-disabled.
	isDisabled(): boolean {
		let org: any;
		try {
			org = this.httpService.getOrganisation();
		} catch {
			return false;
		}
		if (!org) return false;
		return org.disabled_at != null && org.disabled_at !== undefined;
	}

	// Resolve the billing strategy from config. Fail-closed to 'oss' on any error.
	async getBillingStrategy(): Promise<BillingStrategy> {
		try {
			const config = await this.configService.getConfig();
			return config.billing_strategy || 'oss';
		} catch {
			return 'oss';
		}
	}

	disabledOrganisationMessage(strategy: BillingStrategy): string {
		if (strategy === 'cloud') {
			return 'This action is disabled for this organization. Subscribe to a plan to create projects.';
		}

		return 'This action is disabled for this organization. Please contact support.';
	}
}
