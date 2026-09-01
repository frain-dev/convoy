import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';

import { AdminService } from 'src/app/private/pages/admin/admin.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

import { QueueMonitoringAsynqmonComponent } from './queue-monitoring-asynqmon.component';
import { QueueMonitoringBrokerComponent } from './queue-monitoring-broker.component';
import { QueueMonitoringDataplaneComponent } from './queue-monitoring-dataplane.component';

type QueueMonitoringSurface = 'loading' | 'postgres' | 'redis';

@Component({
	selector: 'convoy-queue-monitoring',
	imports: [CommonModule, QueueMonitoringAsynqmonComponent, QueueMonitoringBrokerComponent, QueueMonitoringDataplaneComponent],
	templateUrl: './queue-monitoring.component.html'
})
export class QueueMonitoringComponent implements OnInit {
	licensed = false;
	surface: QueueMonitoringSurface = 'loading';

	constructor(
		private readonly adminService: AdminService,
		private readonly licenses: LicensesService,
		private readonly rbacService: RbacService,
		private readonly router: Router
	) {}

	async ngOnInit(): Promise<void> {
		const role = await this.rbacService.getUserRole();
		if (role !== 'INSTANCE_ADMIN') {
			this.router.navigateByUrl('/');
			return;
		}

		await this.licenses.loadAllLicenses();
		this.licensed = this.licenses.hasInstanceLicense('AsynqMonitoring');
		if (!this.licensed) return;

		await this.resolveSurface();
	}

	get usePostgresUi(): boolean {
		return this.surface === 'postgres';
	}

	// Redis keeps the existing asynqmon iframe. Postgres uses the native broker
	// page because asynqmon reads Redis directly and is not wired for queue_jobs.
	private async resolveSurface(): Promise<void> {
		this.surface = 'loading';

		try {
			const response = await this.adminService.getQueueStats();
			this.surface = response.data?.provider === 'postgres' ? 'postgres' : 'redis';
		} catch {
			// Fail open to the legacy iframe: Redis deployments always had it, and
			// a transient stats read must not hide asynqmon behind an empty gate.
			this.surface = 'redis';
		}
	}
}
