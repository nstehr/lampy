# Project Lampy 

Project contributors: Ada and Nathan

Phillips Hue light controller that changes colour to let us know if it is time to take out cans and garbage or it is time to
take out the cardboard.

If Lampy is blue, then it is recycling and garbage day!
If Lampy is green, then it is cardboard day!

### Details
In our city (Ottawa, Ontario), there is alternating schedule of when garbage is collected.  On one week garbage
and cans are collected, and the following week it is cardboard.  The city provides an ical
for collection days, and that is used as the main input into the application.

The events are parsed out of the provided ical file, and based on the summary of the event, 
we'll use the Phillips Hue API to adjust the colour of the light.  Brightness of the light
is relative to how close we are to garbage+cans or cardboard recycling day.

There are some assumptions made to work with my local city, but shouldn't
be too hard to adjust if you want to make work for your own ical data.

It also shouldn't be too hard to extend past my initial use case to _any_ type of event that
can be pulled out of an ical calendar.  