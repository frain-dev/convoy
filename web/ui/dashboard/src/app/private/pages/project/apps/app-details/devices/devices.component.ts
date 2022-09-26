import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute } from '@angular/router';
import { DEVICE } from 'src/app/models/app.model';
import { CardComponent } from 'src/app/components/card/card.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { DevicesService } from './devices.service';

@Component({
	selector: 'convoy-devices',
	standalone: true,
	imports: [CommonModule, CardComponent, SkeletonLoaderComponent, TagComponent, EmptyStateComponent, StatusColorModule],
	templateUrl: './devices.component.html',
	styleUrls: ['./devices.component.scss']
})
export class DevicesComponent implements OnInit {
	appId: string = this.route.snapshot.params.id;
	isFetchingDevices = false;
	isloadingAppPortalAppDetails = false;
	showError = false;
	devices!: DEVICE[];
	loaderIndex: number[] = [0, 1, 2];
	token: string = this.route.snapshot.params.token;

	constructor(private route: ActivatedRoute, private deviceService: DevicesService) {}

	ngOnInit() {
		this.token ? this.getAppPortalApp() : this.getDevices();
	}

	async getAppPortalApp() {
		this.showError = false;
		this.isloadingAppPortalAppDetails = true;

		try {
			const app = await this.deviceService.getAppPortalApp(this.token);
			this.appId = app.data.uid;
			this.getDevices();
			return;
		} catch (error) {
			this.showError = true;
			this.isloadingAppPortalAppDetails = false;
			return error;
		}
	}
	async getDevices() {
		this.showError = false;
		this.isFetchingDevices = true;
		try {
			const response = await this.deviceService.getAppDevices(this.appId, this.token);
			this.devices = response.data.content;
			this.isFetchingDevices = false;
		} catch {
			this.showError = true;
			this.isFetchingDevices = false;
			return;
		}
	}
}
