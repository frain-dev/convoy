import {Component, OnInit} from '@angular/core';
import {CommonModule} from '@angular/common';
import {EventDeliveriesModule} from 'src/app/private/pages/project/events/event-deliveries/event-deliveries.module';
import {ButtonComponent} from '../../components/button/button.component';
import {CardComponent} from '../../components/card/card.component';
import {ChartComponent} from '../../components/chart/chart.component';
import {DatePickerComponent} from '../../components/date-picker/date-picker.component';
import {DropdownComponent, DropdownOptionDirective} from '../../components/dropdown/dropdown.component';
import {ListItemComponent} from '../../components/list-item/list-item.component';
import {SkeletonLoaderComponent} from '../../components/skeleton-loader/skeleton-loader.component';
import {format} from 'date-fns';
import {CHARTDATA, HTTP_RESPONSE, PAGINATION} from '../../models/global.model';
import {EVENT_DELIVERY} from '../../models/event.model';
import {FormBuilder, FormGroup} from '@angular/forms';
import {EventsService} from '../../private/pages/project/events/events.service';
import {Router} from '@angular/router';

@Component({
    selector: 'convoy-event-deliveries',
    standalone: true,
    imports: [CommonModule, EventDeliveriesModule, ButtonComponent, CardComponent, ChartComponent, DatePickerComponent, DropdownComponent, DropdownOptionDirective, ListItemComponent, SkeletonLoaderComponent],
    templateUrl: './event-deliveries.component.html',
    styleUrls: ['./event-deliveries.component.scss']
})
export class EventDeliveriesComponent implements OnInit {
    isloadingDashboardData: boolean = false;
    dashboardFrequency: 'daily' | 'weekly' | 'monthly' | 'yearly' = 'daily';
    filterOptions: ['daily', 'weekly', 'monthly', 'yearly'] = ['daily', 'weekly', 'monthly', 'yearly'];
    dashboardData = {apps: 0, events_sent: 0};
    eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
    statsDateRange: FormGroup = this.formBuilder.group({
        startDate: [{value: new Date(new Date().setDate(new Date().getDate() - 30)), disabled: true}],
        endDate: [{value: new Date(), disabled: true}]
    });
    dateRangeValue?: {
        startDate: string | Date;
        endDate: string | Date;
    };
    chartData!: CHARTDATA[];
    isPageLoading = false;

    constructor(private formBuilder: FormBuilder, private eventsService: EventsService, public router: Router) {
    }


    async ngOnInit() {
        this.isloadingDashboardData = true;
        this.isPageLoading = true;
        this.fetchDashboardData();
    }

    setDateForFilter(requestDetails: { startDate: Date; endDate: Date; startTime?: string; endTime?: string }) {
        if (!requestDetails.endDate && !requestDetails.startDate) return {startDate: '', endDate: ''};
        const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
        const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
        return {startDate, endDate};
    }

    async fetchDashboardData() {
        const setDate = typeof this.statsDateRange.value.startDate !== 'string';

        const {
            startDate,
            endDate
        } = setDate ? this.setDateForFilter(this.statsDateRange.value) : this.statsDateRange.value;

        try {
            const dashboardResponse = await this.eventsService.dashboardSummary({
                startDate,
                endDate,
                type: this.dashboardFrequency
            });
            this.dashboardData = dashboardResponse.data;
            this.initConvoyChart(dashboardResponse);

            this.isloadingDashboardData = false;
            return;
        } catch (error: any) {
            this.isloadingDashboardData = false;
            this.isPageLoading = false;
            return;
        }
    }

    getSelectedDateRange(dateRange?: { startDate: Date; endDate: Date }) {
        this.dateRangeValue = dateRange;
        this.statsDateRange.patchValue({
            startDate: dateRange?.startDate || new Date(new Date().setDate(new Date().getDate() - 30)),
            endDate: dateRange?.endDate || new Date()
        });
        this.fetchDashboardData();
    }

    initConvoyChart(dashboardResponse: HTTP_RESPONSE) {
        let chartData: { label: string; data: any }[] = [];

        const eventData = dashboardResponse.data.event_data.reverse();
        const labelFormat = this.getDateLabelFormat();
        eventData.forEach((data: any) => {
            chartData.push({
                label: format(new Date(data.data.date), labelFormat),
                data: data.count || 0
            });
        });

        this.chartData = chartData;
    }

    getDateLabelFormat() {
        let labelsDateFormat = '';
        switch (this.dashboardFrequency) {
            case 'daily':
                labelsDateFormat = 'do, MMM, yyyy';
                break;
            case 'weekly':
                labelsDateFormat = 'yyyy-MM';
                break;
            case 'monthly':
                labelsDateFormat = 'MMM, yyyy';
                break;
            case 'yearly':
                labelsDateFormat = 'yyyy';
                break;
            default:
                break;
        }

        return labelsDateFormat;
    }

}
