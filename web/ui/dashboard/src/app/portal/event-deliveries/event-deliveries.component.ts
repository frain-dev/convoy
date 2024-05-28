import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { EventDeliveriesModule } from 'src/app/private/pages/project/events/event-deliveries/event-deliveries.module';

@Component({
  selector: 'convoy-event-deliveries',
  standalone: true,
  imports: [CommonModule, EventDeliveriesModule],
  templateUrl: './event-deliveries.component.html',
  styleUrls: ['./event-deliveries.component.scss']
})
export class EventDeliveriesComponent implements OnInit {

  constructor() { }

  ngOnInit(): void {
  }

}
