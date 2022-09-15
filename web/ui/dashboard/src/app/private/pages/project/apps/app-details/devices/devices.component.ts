import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AppDetailsService } from '../app-details.service';
import { ActivatedRoute } from '@angular/router';
import { DEVICE } from 'src/app/models/app.model';
import { CardComponent } from 'src/app/components/card/card.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';

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
	devices!: DEVICE[];
	loaderIndex: number[] = [0, 1, 2];

	constructor(private route: ActivatedRoute, private appDetailsService: AppDetailsService) {}

	ngOnInit() {
		this.getDevices();
	}

	async getDevices() {
		this.isFetchingDevices = true;
		try {
			const response = await this.appDetailsService.getAppDevices(this.appId);
			this.devices = response.data.content;
			this.isFetchingDevices = false;
		} catch {
			this.isFetchingDevices = false;
			return;
		}
	}
}
